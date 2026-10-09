"""Fork-only exact-source verification; never modifies the candidate commit."""
import collections
import hashlib
import json
import os
import pathlib
import platform
import shutil
import subprocess
import sys

HEAD = "f069b4357d0fe1256f964d676a5cc7d5188a9d9f"
BASE = "d8db233cf9f64bc43bd8b25f7923055260c36762"
source = pathlib.Path(sys.argv[1]).resolve()
receipts = pathlib.Path(sys.argv[2]).resolve()
receipts.mkdir(parents=True, exist_ok=True)
backend = source / "backend"
records = []


def run(label, args, cwd=backend, tests=False, red=None):
    print(f"COMMAND {label}: {args!r}", flush=True)
    p = subprocess.run(args, cwd=cwd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                       encoding="utf-8", errors="replace", timeout=1200)
    (receipts / (label + ".log")).write_text(p.stdout, encoding="utf-8")
    print(p.stdout, flush=True)
    record = {"label": label, "command": args, "cwd": str(cwd), "exit_code": p.returncode}
    if tests:
        events = []
        for line in p.stdout.splitlines():
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                pass
        completed = [e for e in events if e.get("Test") and e.get("Action") in ("pass", "fail", "skip")]
        record["counts"] = dict(collections.Counter(e["Action"] for e in completed))
        record["failed_tests"] = [e["Test"] for e in completed if e["Action"] == "fail"]
        record["skipped_tests"] = [e["Test"] for e in completed if e["Action"] == "skip"]
        assert completed, f"{label}: no tests executed"
    records.append(record)
    (receipts / "results.json").write_text(json.dumps(records, indent=2), encoding="utf-8")
    if red:
        assert p.returncode != 0 and red in record["failed_tests"], f"{label}: expected behavioral RED absent"
        assert "[build failed]" not in p.stdout, f"{label}: compile failure is not RED"
    else:
        assert p.returncode == 0, f"{label}: failed ({p.returncode})"
    return p.stdout.strip()


def clean():
    assert run("head", ["git", "rev-parse", "HEAD"], source) == HEAD
    assert not run("clean", ["git", "status", "--porcelain"], source)


assert platform.system() == "Windows", platform.platform()
identity = {"platform": platform.platform(), "system": platform.system(), "machine": platform.machine(),
            "runner_os": os.getenv("RUNNER_OS"), "workflow_sha": os.getenv("GITHUB_SHA"),
            "tested_source_sha": HEAD, "base_sha": BASE}
print("NATIVE_IDENTITY " + json.dumps(identity), flush=True)
(receipts / "identity.json").write_text(json.dumps(identity, indent=2), encoding="utf-8")
clean()
# Preserve canonical workspace module selection without Go writing go.work.sum
# into the immutable checkout. No dependency versions or assertions change.
workspace_dir = receipts / "go-workspace"
workspace_dir.mkdir(exist_ok=True)
workspace_text = (source / "go.work").read_text(encoding="utf-8")
for module in ("backend", "cloud"):
    workspace_text = workspace_text.replace("./" + module, json.dumps((source / module).as_posix()))
(workspace_dir / "go.work").write_text(workspace_text, encoding="utf-8")
shutil.copyfile(source / "go.work.sum", workspace_dir / "go.work.sum")
os.environ["GOWORK"] = str(workspace_dir / "go.work")
run("go-version", ["go", "version"])
assert run("goos", ["go", "env", "GOOS"]) == "windows"
run("go-env", ["go", "env", "GOOS", "GOARCH", "CGO_ENABLED", "CC", "GOVERSION", "GOWORK"])
manager = source / "backend/internal/session_manager/manager.go"
original = manager.read_bytes()
try:
    # Exact candidate tests, original implementation: catches the missing call.
    manager.write_bytes(subprocess.check_output(["git", "show", BASE + ":backend/internal/session_manager/manager.go"], cwd=source))
    run("red-original-manager", ["go", "test", "-race", "-count=1", "-timeout=5m", "-json", "-run",
        "^TestRetireForReplacement(StopsChatController|DoesNotTerminateWhenChatControllerCannotStop|ChatStopPrecedesWorkspaceTeardown)$",
        "./internal/session_manager"], tests=True, red="TestRetireForReplacementStopsChatController")
finally:
    manager.write_bytes(original)
clean()
run("green-retirement", ["go", "test", "-race", "-count=1", "-timeout=5m", "-json", "-run", "^TestRetireForReplacement", "./internal/session_manager"], tests=True)
run("green-workspace-kill", ["go", "test", "-race", "-count=1", "-timeout=10m", "-json", "-run", "Destroy|Discard|Sweep|Kill", "./internal/adapters/workspace/...", "./internal/session_manager"], tests=True)
run("green-chat-stop", ["go", "test", "-race", "-count=1", "-timeout=5m", "-json", "-run", "^Test(ServiceStop|ControllerCloseHonorsContext|StopClearsHibernation|FailedHibernate|StartWaitsForStoppedController)", "./internal/service/chat"], tests=True)
run("green-provider-job", ["go", "test", "-race", "-count=1", "-timeout=5m", "-json", "-run", "^TestWindowsProviderJobReapsGrandchild$", "./internal/adapters/chatdriver/persistenthost"], tests=True)
run("green-provider-hibernate", ["go", "test", "-race", "-count=1", "-timeout=5m", "-json", "-run", "^(TestPersistentClaudeACPHibernateAndNativeResume|TestCodexHibernateStopsAppServerAndNativeResumesThread)$", "./internal/adapters/chatdriver/acp", "./internal/adapters/chatdriver/codexappserver"], tests=True)
# External validation fixture is copied only after exact-commit suites finish.
fixture = pathlib.Path(__file__).with_name("child_worktree_windows_test.go")
target = source / "backend/internal/session_manager/wave_b_child_windows_test.go"
assert not target.exists()
try:
    shutil.copyfile(fixture, target)
    print("EXTERNAL_FIXTURE_SHA256 " + hashlib.sha256(fixture.read_bytes()).hexdigest(), flush=True)
    run("green-external-child-worktree", ["go", "test", "-race", "-count=1", "-timeout=5m", "-json", "-run", "^TestWaveBWindowsRetirementReleasesChildWorktree$", "./internal/session_manager"], tests=True)
    try:
        manager.write_bytes(subprocess.check_output(["git", "show", BASE + ":backend/internal/session_manager/manager.go"], cwd=source))
        run("red-external-child-worktree", ["go", "test", "-race", "-count=1", "-timeout=5m", "-json", "-run", "^TestWaveBWindowsRetirementReleasesChildWorktree$", "./internal/session_manager"], tests=True, red="TestWaveBWindowsRetirementReleasesChildWorktree")
    finally:
        manager.write_bytes(original)
finally:
    target.unlink(missing_ok=True)
clean()
print("NATIVE_WINDOWS_VALIDATION_PASS tested_source_sha=" + HEAD, flush=True)
