use serde::Serialize;
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::Manager;
use tauri_plugin_shell::ShellExt;

struct DaemonState {
    running: AtomicBool,
    port: std::sync::Mutex<Option<u16>>,
}

#[derive(Clone, Serialize)]
struct DaemonInfo {
    port: u16,
}

#[derive(Clone, Serialize)]
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

    let output = app
        .shell()
        .sidecar("daemon")
        .map_err(|e| format!("sidecar error: {}", e))?
        .args(["--port", "0", "--data-dir", &data_dir])
        .output()
        .await
        .map_err(|e| format!("daemon spawn: {}", e))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(format!("daemon failed: {}", stderr));
    }

    let stdout = String::from_utf8_lossy(&output.stdout);
    let port: u16 = stdout
        .trim()
        .parse()
        .map_err(|e| format!("parse port: {}", e))?;

    state.port.lock().unwrap().replace(port);
    state.running.store(true, Ordering::SeqCst);

    log::info!("daemon started on port {}", port);

    Ok(DaemonInfo { port })
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
    let port = state.port.lock().unwrap().take();
    state.running.store(false, Ordering::SeqCst);

    if let Some(port) = port {
        let url = format!("http://127.0.0.1:{}/shutdown", port);
        let _ = reqwest::Client::new().post(&url).send().await;
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
        })
        .invoke_handler(tauri::generate_handler![
            start_daemon,
            check_health,
            shutdown_daemon,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
