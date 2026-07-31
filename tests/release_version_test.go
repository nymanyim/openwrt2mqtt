package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareReleaseVersion(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}

	root := t.TempDir()
	work := filepath.Join(root, "work")
	runCommand(t, root, nil, git, "init", "--initial-branch=main", work)

	writePackageMakefile(t, filepath.Join(work, "package", "openwrt2mqtt", "Makefile"), "1.0.0", "3")
	writePackageMakefile(t, filepath.Join(work, "package", "luci-app-openwrt2mqtt", "Makefile"), "1.0.0", "3")
	script, err := os.ReadFile(repoPath(t, ".github", "scripts", "prepare-release-version.sh"))
	if err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(work, ".github", "scripts", "prepare-release-version.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, script, 0o755); err != nil {
		t.Fatal(err)
	}

	runCommand(t, work, nil, git, "config", "user.name", "Test")
	runCommand(t, work, nil, git, "config", "user.email", "test@example.com")
	runCommand(t, work, nil, git, "add", ".")
	runCommand(t, work, nil, git, "commit", "-m", "initial")
	initialHead := commandOutput(t, work, git, "rev-parse", "HEAD")
	runCommand(t, work, nil, git, "update-ref", "refs/remotes/origin/main", initialHead)

	tagsPath := filepath.Join(root, "tags")
	if err := os.WriteFile(tagsPath, []byte("v1.0.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wrapperDirectory := filepath.Join(root, "bin")
	writeGitWrapper(t, wrapperDirectory)
	baseEnvironment := map[string]string{
		"GITHUB_REF_NAME": "main",
		"MOCK_GIT_TAGS":   tagsPath,
		"REAL_GIT":        git,
		"PATH":            wrapperDirectory + string(os.PathListSeparator) + os.Getenv("PATH"),
	}

	firstOutput := filepath.Join(root, "first-output")
	firstEnvironment := cloneEnvironment(baseEnvironment)
	firstEnvironment["GITHUB_OUTPUT"] = firstOutput
	runCommand(t, work, firstEnvironment, bash, scriptPath)
	assertPackageVersion(t, work, "1.0.1", "1")
	firstHead := commandOutput(t, work, git, "rev-parse", "HEAD")
	if remoteHead := commandOutput(t, work, git, "rev-parse", "origin/main"); remoteHead != firstHead {
		t.Fatalf("remote main = %s, want %s", remoteHead, firstHead)
	}
	if subject := commandOutput(t, work, git, "log", "-1", "--format=%s"); subject != "chore: prepare release v1.0.1" {
		t.Fatalf("commit subject = %q", subject)
	}
	assertVersionOutput(t, firstOutput, "1.0.1", firstHead)

	secondOutput := filepath.Join(root, "second-output")
	secondEnvironment := cloneEnvironment(baseEnvironment)
	secondEnvironment["GITHUB_OUTPUT"] = secondOutput
	runCommand(t, work, secondEnvironment, bash, scriptPath)
	if secondHead := commandOutput(t, work, git, "rev-parse", "HEAD"); secondHead != firstHead {
		t.Fatalf("idempotent run created commit %s, want %s", secondHead, firstHead)
	}
	assertVersionOutput(t, secondOutput, "1.0.1", firstHead)

	if err := os.WriteFile(tagsPath, []byte("v1.0.0\nv1.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	thirdOutput := filepath.Join(root, "third-output")
	thirdEnvironment := cloneEnvironment(baseEnvironment)
	thirdEnvironment["GITHUB_OUTPUT"] = thirdOutput
	runCommand(t, work, thirdEnvironment, bash, scriptPath)
	assertPackageVersion(t, work, "1.0.2", "1")
	thirdHead := commandOutput(t, work, git, "rev-parse", "HEAD")
	if thirdHead == firstHead {
		t.Fatal("next tagged release did not create a version commit")
	}
	assertVersionOutput(t, thirdOutput, "1.0.2", thirdHead)
}

func TestPrepareReleaseVersionRejectsNonMainBranch(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	output, err := exec.Command(bash, repoPath(t, ".github", "scripts", "prepare-release-version.sh")).CombinedOutput()
	if err == nil {
		t.Fatal("release version script accepted a non-main branch")
	}
	if !strings.Contains(string(output), "Release 只能从 main 分支发布。") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestPrepareReleaseVersionRejectsMismatchedPackages(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	work := t.TempDir()
	writePackageMakefile(t, filepath.Join(work, "package", "openwrt2mqtt", "Makefile"), "1.0.0", "3")
	writePackageMakefile(t, filepath.Join(work, "package", "luci-app-openwrt2mqtt", "Makefile"), "1.0.1", "1")

	command := exec.Command(bash, repoPath(t, ".github", "scripts", "prepare-release-version.sh"))
	command.Dir = work
	command.Env = append(os.Environ(), "GITHUB_REF_NAME=main")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("release version script accepted mismatched packages")
	}
	if !strings.Contains(string(output), "核心包和 LuCI 包版本不一致。") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func writeGitWrapper(t *testing.T, directory string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	wrapper := `#!/bin/sh
case "$1" in
ls-remote)
	pattern="$5"
	tag="${pattern##*/}"
	grep -Fxq "$tag" "$MOCK_GIT_TAGS" || exit 2
	printf '%040d\trefs/tags/%s\n' 1 "$tag"
	;;
fetch)
	exit 0
	;;
push)
	"$REAL_GIT" update-ref refs/remotes/origin/main HEAD
	;;
*)
	exec "$REAL_GIT" "$@"
	;;
esac
`
	path := filepath.Join(directory, "git")
	if err := os.WriteFile(path, []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
}

func cloneEnvironment(environment map[string]string) map[string]string {
	clone := make(map[string]string, len(environment))
	for key, value := range environment {
		clone[key] = value
	}
	return clone
}

func writePackageMakefile(t *testing.T, path, version, release string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "PKG_VERSION:=" + version + "\nPKG_RELEASE:=" + release + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertPackageVersion(t *testing.T, work, version, release string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(work, "package", "openwrt2mqtt", "Makefile"),
		filepath.Join(work, "package", "luci-app-openwrt2mqtt", "Makefile"),
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if !strings.Contains(text, "PKG_VERSION:="+version+"\n") || !strings.Contains(text, "PKG_RELEASE:="+release+"\n") {
			t.Fatalf("unexpected package metadata in %s:\n%s", path, text)
		}
	}
}

func assertVersionOutput(t *testing.T, path, version, sourceSHA string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "version="+version+"\n") || !strings.Contains(text, "source_sha="+sourceSHA+"\n") {
		t.Fatalf("unexpected version output:\n%s", text)
	}
}

func commandOutput(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func runCommand(t *testing.T, directory string, environment map[string]string, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	command.Env = os.Environ()
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
}
