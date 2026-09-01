package model

import "time"

type Config struct {
	Resources []Resource `json:"resources"`
}

type Resource struct {
	ID                 string   `json:"id"`
	Type               string   `json:"type"`
	Path               string   `json:"path,omitempty"`
	Source             string   `json:"source,omitempty"`
	Content            *string  `json:"content,omitempty"`
	Owner              string   `json:"owner,omitempty"`
	Group              string   `json:"group,omitempty"`
	Mode               string   `json:"mode,omitempty"`
	Name               string   `json:"name,omitempty"`
	Enabled            *bool    `json:"enabled,omitempty"`
	Running            *bool    `json:"running,omitempty"`
	Requires           []string `json:"requires,omitempty"`
	ReloadWhenChanged  []string `json:"reload_when_changed,omitempty"`
	RestartWhenChanged []string `json:"restart_when_changed,omitempty"`
}

type Plan struct {
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	ConfigDigest string    `json:"config_digest"`
	Changes      []Change  `json:"changes"`
	Source       string    `json:"source"`
}

type Change struct {
	ResourceID string         `json:"resource_id"`
	Type       string         `json:"type"`
	Target     string         `json:"target"`
	Actions    []string       `json:"actions"`
	File       *FileChange    `json:"file,omitempty"`
	Systemd    *SystemdChange `json:"systemd,omitempty"`
	Diff       string         `json:"diff,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

type FileState struct {
	Exists bool   `json:"exists"`
	Digest string `json:"digest,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
	UID    int    `json:"uid,omitempty"`
	GID    int    `json:"gid,omitempty"`
}

type FileChange struct {
	Before  FileState `json:"before"`
	Content []byte    `json:"content"`
	Mode    uint32    `json:"mode"`
	UID     int       `json:"uid"`
	GID     int       `json:"gid"`
}

type SystemdState struct {
	Enabled bool `json:"enabled"`
	Running bool `json:"running"`
}

type SystemdChange struct {
	Before  SystemdState `json:"before"`
	Enabled *bool        `json:"enabled,omitempty"`
	Running *bool        `json:"running,omitempty"`
}
