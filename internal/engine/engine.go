package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/zawachte/huddle/internal/config"
	"github.com/zawachte/huddle/internal/model"
)

func BuildPlan(source string, cfg model.Config, raw []byte) (model.Plan, error) {
	plan := model.Plan{Version: 1, CreatedAt: time.Now().UTC(), ConfigDigest: config.Digest(raw), Source: source}
	changed := map[string]bool{}
	for _, r := range cfg.Resources {
		var change model.Change
		var err error
		switch r.Type {
		case "file":
			change, err = planFile(r)
		case "systemd":
			change, err = planSystemd(r, changed)
		}
		if err != nil {
			return model.Plan{}, fmt.Errorf("plan %s: %w", r.ID, err)
		}
		if len(change.Actions) > 0 {
			plan.Changes = append(plan.Changes, change)
			changed[r.ID] = true
		}
	}
	return plan, nil
}

func planFile(r model.Resource) (model.Change, error) {
	desired, err := desiredContent(r)
	if err != nil {
		return model.Change{}, err
	}
	state, current, err := inspectFile(r.Path)
	if err != nil {
		return model.Change{}, err
	}
	mode, err := desiredMode(r.Mode, state)
	if err != nil {
		return model.Change{}, err
	}
	uid, gid, err := desiredOwnership(r.Owner, r.Group, state)
	if err != nil {
		return model.Change{}, err
	}
	change := model.Change{ResourceID: r.ID, Type: "file", Target: r.Path}
	if !state.Exists {
		change.Actions = append(change.Actions, "create")
	} else if digest(current) != digest(desired) {
		change.Actions = append(change.Actions, "update-content")
	}
	if state.Exists && state.Mode != mode {
		change.Actions = append(change.Actions, "chmod")
	}
	if state.Exists && (state.UID != uid || state.GID != gid) {
		change.Actions = append(change.Actions, "chown")
	}
	if len(change.Actions) > 0 {
		change.File = &model.FileChange{Before: state, Content: desired, Mode: mode, UID: uid, GID: gid}
		if state.Exists && string(current) != string(desired) {
			change.Diff = unified(current, desired, r.Path)
		}
	}
	return change, nil
}

func planSystemd(r model.Resource, changed map[string]bool) (model.Change, error) {
	state, err := inspectSystemd(r.Name)
	if err != nil {
		return model.Change{}, err
	}
	change := model.Change{ResourceID: r.ID, Type: "systemd", Target: r.Name, Systemd: &model.SystemdChange{Before: state, Enabled: r.Enabled, Running: r.Running}}
	if r.Enabled != nil && *r.Enabled != state.Enabled {
		if *r.Enabled {
			change.Actions = append(change.Actions, "enable")
		} else {
			change.Actions = append(change.Actions, "disable")
		}
	}
	if r.Running != nil && *r.Running != state.Running {
		if *r.Running {
			change.Actions = append(change.Actions, "start")
		} else {
			change.Actions = append(change.Actions, "stop")
		}
	}
	for _, id := range r.ReloadWhenChanged {
		if changed[id] {
			change.Actions = append(change.Actions, "reload")
			change.Reason = "triggered by " + id
			break
		}
	}
	for _, id := range r.RestartWhenChanged {
		if changed[id] {
			change.Actions = append(change.Actions, "restart")
			change.Reason = "triggered by " + id
			break
		}
	}
	if len(change.Actions) == 0 {
		change.Systemd = nil
	}
	return change, nil
}

func Apply(plan model.Plan) error {
	for _, c := range plan.Changes {
		switch c.Type {
		case "file":
			if err := applyFile(c); err != nil {
				return fmt.Errorf("apply %s: %w", c.ResourceID, err)
			}
		case "systemd":
			if err := applySystemd(c); err != nil {
				return fmt.Errorf("apply %s: %w", c.ResourceID, err)
			}
		default:
			return fmt.Errorf("plan contains unsupported type %q", c.Type)
		}
	}
	return nil
}

func applyFile(c model.Change) error {
	if c.File == nil {
		return fmt.Errorf("missing file operation")
	}
	state, _, err := inspectFile(c.Target)
	if err != nil {
		return err
	}
	if state != c.File.Before {
		return fmt.Errorf("stale plan: current state no longer matches planned state")
	}
	dir := filepath.Dir(c.Target)
	tmp, err := os.CreateTemp(dir, ".huddle-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(c.File.Content); err == nil {
		err = tmp.Chmod(os.FileMode(c.File.Mode))
	}
	if err == nil {
		err = tmp.Chown(c.File.UID, c.File.GID)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, c.Target)
	}
	if err != nil {
		return err
	}
	ok = true
	return nil
}

func applySystemd(c model.Change) error {
	if c.Systemd == nil {
		return fmt.Errorf("missing systemd operation")
	}
	state, err := inspectSystemd(c.Target)
	if err != nil {
		return err
	}
	if state != c.Systemd.Before {
		return fmt.Errorf("stale plan: service state no longer matches planned state")
	}
	for _, action := range c.Actions {
		cmd := exec.Command("systemctl", action, c.Target)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl %s: %s: %w", action, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func inspectFile(path string) (model.FileState, []byte, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return model.FileState{}, nil, nil
	}
	if err != nil {
		return model.FileState{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return model.FileState{}, nil, err
	}
	uid, gid := statOwnership(info)
	return model.FileState{Exists: true, Digest: digest(data), Mode: uint32(info.Mode().Perm()), UID: uid, GID: gid}, data, nil
}

func inspectSystemd(name string) (model.SystemdState, error) {
	load := exec.Command("systemctl", "show", name, "--property=LoadState", "--value")
	out, err := load.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "loaded" {
		return model.SystemdState{}, fmt.Errorf("unit %s is not loaded", name)
	}
	enabled := exec.Command("systemctl", "is-enabled", "--quiet", name).Run() == nil
	running := exec.Command("systemctl", "is-active", "--quiet", name).Run() == nil
	return model.SystemdState{Enabled: enabled, Running: running}, nil
}

func desiredContent(r model.Resource) ([]byte, error) {
	if r.Content != nil {
		return []byte(*r.Content), nil
	}
	return os.ReadFile(r.Source)
}

func desiredMode(value string, current model.FileState) (uint32, error) {
	if value == "" {
		if current.Exists {
			return current.Mode, nil
		}
		return 0644, nil
	}
	n, err := strconv.ParseUint(value, 8, 32)
	if err != nil || n > 07777 {
		return 0, fmt.Errorf("invalid mode %q", value)
	}
	return uint32(n), nil
}

func desiredOwnership(owner, group string, current model.FileState) (int, int, error) {
	uid, gid := current.UID, current.GID
	if !current.Exists {
		uid, gid = os.Geteuid(), os.Getegid()
	}
	if owner != "" {
		u, err := user.Lookup(owner)
		if err != nil {
			return 0, 0, err
		}
		uid64, _ := strconv.ParseInt(u.Uid, 10, 32)
		uid = int(uid64)
	}
	if group != "" {
		g, err := user.LookupGroup(group)
		if err != nil {
			return 0, 0, err
		}
		gid64, _ := strconv.ParseInt(g.Gid, 10, 32)
		gid = int(gid64)
	}
	return uid, gid, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func unified(before, after []byte, path string) string {
	d, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: difflib.SplitLines(string(before)), B: difflib.SplitLines(string(after)), FromFile: path + " (current)", ToFile: path + " (planned)", Context: 3})
	return strings.TrimSpace(d)
}
