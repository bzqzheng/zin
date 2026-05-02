use std::env;
use std::path::PathBuf;
use std::process::Command;

fn main() {
    build_daemon_sidecar();
    tauri_build::build();
}

fn build_daemon_sidecar() {
    println!("cargo:rerun-if-changed=../../../services/daemon");

    let target = env::var("TARGET").expect("TARGET is set by cargo");
    let manifest_dir =
        PathBuf::from(env::var("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR set"));
    let repo_root = manifest_dir
        .parent()
        .and_then(|p| p.parent())
        .and_then(|p| p.parent())
        .expect("src-tauri is nested under apps/desktop");

    let daemon_dir = repo_root.join("services/daemon/cmd/daemon");
    let sidecar_path = daemon_dir.join(format!("daemon-{}", target));

    let mut command = Command::new("go");
    command.arg("build").arg("-o").arg(&sidecar_path).arg(".");

    if let Some((goos, goarch)) = go_target(&target) {
        command.env("GOOS", goos).env("GOARCH", goarch);
    }

    let status = command
        .current_dir(&daemon_dir)
        .status()
        .expect("failed to run go build for daemon sidecar");

    if !status.success() {
        panic!(
            "failed to build daemon sidecar at {}",
            sidecar_path.display()
        );
    }
}

fn go_target(target: &str) -> Option<(&'static str, &'static str)> {
    let goos = if target.contains("apple-darwin") {
        "darwin"
    } else if target.contains("linux") {
        "linux"
    } else if target.contains("windows") {
        "windows"
    } else {
        return None;
    };

    let goarch = if target.starts_with("aarch64") {
        "arm64"
    } else if target.starts_with("x86_64") {
        "amd64"
    } else {
        return None;
    };

    Some((goos, goarch))
}
