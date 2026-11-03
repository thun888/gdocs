package showdoc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Git struct {
	Dir        string
	ClearProxy bool
}

func NewGit(dir string, clearProxy bool) *Git {
	return &Git{Dir: dir, ClearProxy: clearProxy}
}

func (g *Git) Init(ctx context.Context, name, email string) error {
	if err := os.MkdirAll(g.Dir, 0o755); err != nil {
		return err
	}
	if _, err := g.run(ctx, g.Dir, "init", "-b", "main"); err != nil {
		return err
	}
	settings := [][2]string{
		{"core.autocrlf", "false"},
		{"core.quotepath", "false"},
		{"commit.gpgsign", "false"},
		{"user.name", name},
		{"user.email", email},
	}
	for _, item := range settings {
		if _, err := g.run(ctx, g.Dir, "config", item[0], item[1]); err != nil {
			return err
		}
	}
	return nil
}

func (g *Git) RemoteHasBranch(ctx context.Context, url, branch string) (bool, error) {
	out, err := g.run(ctx, "", "ls-remote", url, "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (g *Git) CommitFile(ctx context.Context, rel string, data []byte, author, email string, at time.Time, message string) error {
	full := filepath.Join(g.Dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return err
	}
	if _, err := g.run(ctx, g.Dir, "add", "--", rel); err != nil {
		return err
	}
	return g.commit(ctx, author, email, at, message)
}

func (g *Git) CommitAll(ctx context.Context, author, email string, at time.Time, message string) error {
	if _, err := g.run(ctx, g.Dir, "add", "-A"); err != nil {
		return err
	}
	changed, err := g.hasStaged(ctx)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return g.commit(ctx, author, email, at, message)
}

func (g *Git) Push(ctx context.Context, url, branch string, force bool) error {
	args := []string{"push"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, url, "HEAD:refs/heads/"+branch)
	_, err := g.run(ctx, g.Dir, args...)
	return err
}

func (g *Git) commit(ctx context.Context, author, email string, at time.Time, message string) error {
	args := []string{"commit", "--no-verify", "--quiet", "-m", message}
	_, err := g.runWithEnv(ctx, g.Dir, args, map[string]string{
		"GIT_AUTHOR_NAME":     author,
		"GIT_AUTHOR_EMAIL":    email,
		"GIT_AUTHOR_DATE":     gitDate(at),
		"GIT_COMMITTER_NAME":  author,
		"GIT_COMMITTER_EMAIL": email,
		"GIT_COMMITTER_DATE":  gitDate(at),
	})
	return err
}

func (g *Git) hasStaged(ctx context.Context) (bool, error) {
	cmd := g.command(ctx, g.Dir, "diff", "--cached", "--quiet")
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("检查暂存区: %w", err)
}

func (g *Git) run(ctx context.Context, dir string, args ...string) (string, error) {
	return g.runWithEnv(ctx, dir, args, nil)
}

func (g *Git) runWithEnv(ctx context.Context, dir string, args []string, extra map[string]string) (string, error) {
	cmd := g.command(ctx, dir, args...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	for key, value := range extra {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			msg = strings.TrimSpace(msg + " " + detail)
		}
		if msg != "" {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func (g *Git) command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	full := make([]string, 0, len(args)+4)
	if g.ClearProxy {
		full = append(full, "-c", "http.proxy=")
	}
	if dir != "" {
		full = append(full, "-C", dir)
	}
	full = append(full, args...)
	return exec.CommandContext(ctx, "git", full...)
}

func gitDate(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.In(cst).Format("2006-01-02T15:04:05-07:00")
}
