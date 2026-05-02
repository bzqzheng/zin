use serde::{Deserialize, Serialize};
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::Manager;
use tauri_plugin_shell::process::CommandEvent;
use tauri_plugin_shell::ShellExt;

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

    let (mut rx, child) = app
        .shell()
        .sidecar("daemon")
        .map_err(|e| format!("sidecar error: {}", e))?
        .args(["--port", "0", "--data-dir", &data_dir])
        .spawn()
        .map_err(|e| format!("daemon spawn: {}", e))?;

    log::info!("daemon spawned, pid: {}", child.pid());

    let mut port: u16 = 0;
    while let Some(event) = rx.recv().await {
        match event {
            CommandEvent::Stdout(line) => {
                let text = String::from_utf8_lossy(&line);
                log::info!("daemon stdout: {}", text);
                if port == 0 {
                    if let Ok(p) = text.trim().parse::<u16>() {
                        port = p;
                        break;
                    }
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

    if port == 0 {
        return Err("daemon did not report port".to_string());
    }

    state.port.lock().unwrap().replace(port);
    state.child.lock().unwrap().replace(child);
    state.running.store(true, Ordering::SeqCst);

    log::info!("daemon started on port {}", port);

    let pid = state.child.lock().unwrap().as_ref().map(|c| c.pid()).unwrap_or(0);

    Ok(DaemonInfo { port, pid })
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
    state.running.store(false, Ordering::SeqCst);

    let port = state.port.lock().unwrap().take();
    if let Some(port) = port {
        let url = format!("http://127.0.0.1:{}/shutdown", port);
        let _ = reqwest::Client::new().post(&url).send().await;
    }

    if let Some(child) = state.child.lock().unwrap().take() {
        let _ = child.kill();
    }

    Ok(())
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
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
