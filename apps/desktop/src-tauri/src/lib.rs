use serde::{Deserialize, Serialize};
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;
use tauri::Manager;
use tauri_plugin_shell::process::CommandEvent;
use tauri_plugin_shell::ShellExt;

const DAEMON_STARTUP_TIMEOUT: Duration = Duration::from_secs(5);
const DAEMON_HEALTH_TIMEOUT: Duration = Duration::from_secs(1);
const DAEMON_SHUTDOWN_TIMEOUT: Duration = Duration::from_secs(2);

struct DaemonState {
    running: AtomicBool,
    port: std::sync::Mutex<Option<u16>>,
    child: std::sync::Mutex<Option<tauri_plugin_shell::process::CommandChild>>,
}

#[derive(Clone, Serialize)]
struct DaemonInfo {
    port: u16,
    pid: u32,
}

#[derive(Clone, Serialize, Deserialize)]
struct HealthStatus {
    status: String,
    uptime_seconds: f64,
    db_status: String,
    version: String,
    port: u16,
}

#[tauri::command]
async fn start_daemon(
    app: tauri::AppHandle,
    state: tauri::State<'_, DaemonState>,
) -> Result<DaemonInfo, String> {
    let data_dir = app
        .path()
        .app_data_dir()
        .map_err(|e| format!("app data dir: {}", e))?
        .to_string_lossy()
        .to_string();

    loop {
        if state
            .running
            .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
            .is_ok()
        {
            break;
        }

        if let Some(info) = current_daemon_info(&state) {
            if daemon_is_healthy(info.port).await {
                return Ok(info);
            }

            log::warn!("cached daemon is not healthy; cleaning up before restart");
            cleanup_daemon(&state).await;
            continue;
        }

        return Err("daemon startup already in progress".to_string());
    }

    let command = match app.shell().sidecar("daemon") {
        Ok(command) => command,
        Err(err) => {
            state.running.store(false, Ordering::SeqCst);
            return Err(format!("sidecar error: {}", err));
        }
    };

    let (mut rx, child) = match command
        .args(["--port", "0", "--data-dir", &data_dir])
        .spawn()
    {
        Ok(spawned) => spawned,
        Err(err) => {
            state.running.store(false, Ordering::SeqCst);
            return Err(format!("daemon spawn: {}", err));
        }
    };

    log::info!("daemon spawned, pid: {}", child.pid());
    state.child.lock().unwrap().replace(child);

    let port = match tokio::time::timeout(DAEMON_STARTUP_TIMEOUT, async {
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    let text = String::from_utf8_lossy(&line);
                    log::info!("daemon stdout: {}", text);
                    if let Ok(port) = text.trim().parse::<u16>() {
                        return Ok(port);
                    }
                }
                CommandEvent::Stderr(line) => {
                    let text = String::from_utf8_lossy(&line);
                    log::warn!("daemon stderr: {}", text);
                }
                CommandEvent::Terminated(status) => {
                    log::error!("daemon exited early: {:?}", status);
                    return Err(format!("daemon exited early: {:?}", status));
                }
                _ => {}
            }
        }

        Err("daemon did not report port".to_string())
    })
    .await
    {
        Ok(Ok(port)) => port,
        Ok(Err(err)) => {
            cleanup_daemon(&state).await;
            return Err(err);
        }
        Err(_) => {
            cleanup_daemon(&state).await;
            return Err("daemon startup timed out".to_string());
        }
    };

    state.port.lock().unwrap().replace(port);

    log::info!("daemon started on port {}", port);

    current_daemon_info(&state).ok_or("daemon child missing after startup".to_string())
}

#[tauri::command]
async fn check_health(state: tauri::State<'_, DaemonState>) -> Result<HealthStatus, String> {
    let port = state.port.lock().unwrap().ok_or("daemon not running")?;

    let url = format!("http://127.0.0.1:{}/health", port);
    let resp = reqwest::get(&url)
        .await
        .map_err(|e| format!("health check: {}", e))?;

    let health: HealthStatus = resp
        .json()
        .await
        .map_err(|e| format!("parse health: {}", e))?;

    Ok(health)
}

#[tauri::command]
async fn shutdown_daemon(state: tauri::State<'_, DaemonState>) -> Result<(), String> {
    cleanup_daemon(&state).await;
    Ok(())
}

fn current_daemon_info(state: &DaemonState) -> Option<DaemonInfo> {
    let port = state.port.lock().unwrap().as_ref().copied()?;
    let pid = state.child.lock().unwrap().as_ref().map(|c| c.pid())?;

    Some(DaemonInfo { port, pid })
}

async fn daemon_is_healthy(port: u16) -> bool {
    let url = format!("http://127.0.0.1:{}/health", port);
    let request = reqwest::Client::new().get(&url).send();

    match tokio::time::timeout(DAEMON_HEALTH_TIMEOUT, request).await {
        Ok(Ok(resp)) => resp.status().is_success(),
        Ok(Err(err)) => {
            log::warn!("daemon health probe failed: {}", err);
            false
        }
        Err(_) => {
            log::warn!("daemon health probe timed out");
            false
        }
    }
}

async fn cleanup_daemon(state: &DaemonState) {
    state.running.store(false, Ordering::SeqCst);

    let port = state.port.lock().unwrap().take();
    if let Some(port) = port {
        let url = format!("http://127.0.0.1:{}/shutdown", port);
        let request = reqwest::Client::new().post(&url).send();
        match tokio::time::timeout(DAEMON_SHUTDOWN_TIMEOUT, request).await {
            Ok(Ok(_)) => {}
            Ok(Err(err)) => log::warn!("daemon graceful shutdown failed: {}", err),
            Err(_) => log::warn!("daemon graceful shutdown timed out"),
        }
    }

    if let Some(child) = state.child.lock().unwrap().take() {
        let _ = child.kill();
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
            if cfg!(debug_assertions) {
                app.handle().plugin(
                    tauri_plugin_log::Builder::default()
                        .level(log::LevelFilter::Info)
                        .build(),
                )?;
            }
            Ok(())
        })
        .manage(DaemonState {
            running: AtomicBool::new(false),
            port: std::sync::Mutex::new(None),
            child: std::sync::Mutex::new(None),
        })
        .invoke_handler(tauri::generate_handler![
            start_daemon,
            check_health,
            shutdown_daemon,
        ])
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(|app_handle, event| match event {
            tauri::RunEvent::Exit | tauri::RunEvent::ExitRequested { .. } => {
                let state = app_handle.state::<DaemonState>();
                tauri::async_runtime::block_on(cleanup_daemon(&state));
            }
            _ => {}
        });
}
