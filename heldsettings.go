package main

// heldsettings.go — settings from a restored app backup that are not applied
// until the user confirms them.
//
// An app backup can come from another machine, or be edited. The settings that
// choose which program runs and with what arguments (the burn command, tool
// paths, par2's extra arguments, the tape tool) are therefore not applied by a
// restore: the current values stay, and the backup's are held here until the
// user applies them with the typed word "Confirm" (they then pass the same
// checks as any settings change) or discards them.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

// HeldSettings are a restored backup's program settings, not yet applied.
type HeldSettings struct {
	BurnCommand   string            `json:"burn_command"`
	Tools         map[string]string `json:"tools"`
	Par2ExtraArgs string            `json:"par2_extra_args"`
	TapeTool      string            `json:"tape_tool,omitempty"`
	From          string            `json:"from"`
	At            time.Time         `json:"at"`
}

func (a *App) heldSettingsPath() string { return filepath.Join(a.DataDir, "held-settings.json") }

// holdProgramSettings takes the program settings out of a restored config:
// restored keeps the current ones, and the backup's are returned when they
// differ (nil when there is nothing to hold).
func holdProgramSettings(restored *Config, current Config, from string) *HeldSettings {
	h := &HeldSettings{BurnCommand: restored.BurnCommand, Tools: restored.Tools, Par2ExtraArgs: restored.Par2ExtraArgs, TapeTool: restored.TapeTool, From: from, At: time.Now().UTC()}
	if h.Tools == nil {
		h.Tools = map[string]string{}
	}
	cur := current.Tools
	if cur == nil {
		cur = map[string]string{}
	}
	restored.BurnCommand, restored.Par2ExtraArgs, restored.TapeTool = current.BurnCommand, current.Par2ExtraArgs, current.TapeTool
	restored.Tools = map[string]string{}
	for k, v := range cur {
		restored.Tools[k] = v
	}
	if h.BurnCommand == current.BurnCommand && h.Par2ExtraArgs == current.Par2ExtraArgs && h.TapeTool == current.TapeTool && reflect.DeepEqual(h.Tools, cur) {
		return nil
	}
	return h
}

// saveHeldSettings records held settings, owner-only.
func (a *App) saveHeldSettings(h *HeldSettings) error {
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := writeStage(a.heldSettingsPath(), b, 0o600)
	if err != nil {
		return err
	}
	if err := renameReplacing(tmp, a.heldSettingsPath()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// HeldSettings returns the settings waiting to be applied, or nil.
func (a *App) HeldSettings() (*HeldSettings, error) {
	b, err := os.ReadFile(a.heldSettingsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var h HeldSettings
	if err := json.Unmarshal(b, &h); err != nil {
		return nil, fmt.Errorf("held settings are unreadable: %w", err)
	}
	return &h, nil
}

// ApplyHeldSettings applies the held settings once the user types "Confirm".
// They go through SaveConfig, so every check on these settings applies.
func (a *App) ApplyHeldSettings(confirm string) (Config, error) {
	if confirm != burnCommandConfirmWord {
		return Config{}, fmt.Errorf("type %q to apply the settings from the backup", burnCommandConfirmWord)
	}
	h, err := a.HeldSettings()
	if err != nil {
		return Config{}, err
	}
	if h == nil {
		return Config{}, fmt.Errorf("no settings from a backup are waiting to be applied")
	}
	tools := map[string]any{}
	for k, v := range h.Tools {
		tools[k] = v
	}
	cfg, err := a.SaveConfig(map[string]any{
		"burn_command": h.BurnCommand, "burn_command_confirm": burnCommandConfirmWord,
		"par2_extra_args": h.Par2ExtraArgs, "tape_tool": h.TapeTool, "tools": tools,
	})
	if err != nil {
		return Config{}, fmt.Errorf("the settings from the backup were not applied: %w", err)
	}
	if err := a.DiscardHeldSettings(); err != nil {
		return cfg, err
	}
	a.Store.Log("config", "applied the program settings held from an app-backup restore")
	return cfg, nil
}

// DiscardHeldSettings drops the held settings.
func (a *App) DiscardHeldSettings() error {
	err := os.Remove(a.heldSettingsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
