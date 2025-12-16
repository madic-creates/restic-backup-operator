/*
Copyright 2024 madic-creates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package restic

import (
	"testing"
)

func TestNewCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected []string
	}{
		{"init command", "init", []string{"init"}},
		{"backup command", "backup", []string{"backup"}},
		{"restore command", "restore", []string{"restore"}},
		{"check command", "check", []string{"check"}},
		{"snapshots command", "snapshots", []string{"snapshots"}},
		{"forget command", "forget", []string{"forget"}},
		{"prune command", "prune", []string{"prune"}},
		{"stats command", "stats", []string{"stats"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand(tt.cmd)
			result := cmd.Build()
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
			for i, arg := range tt.expected {
				if result[i] != arg {
					t.Errorf("expected arg %d to be %s, got %s", i, arg, result[i])
				}
			}
		})
	}
}

func TestCommandBuilder_WithJSON(t *testing.T) {
	cmd := NewCommand("snapshots").WithJSON()
	result := cmd.Build()

	expected := []string{"snapshots", "--json"}
	assertArgs(t, expected, result)
}

func TestCommandBuilder_WithQuiet(t *testing.T) {
	cmd := NewCommand("backup").WithQuiet()
	result := cmd.Build()

	expected := []string{"backup", "--quiet"}
	assertArgs(t, expected, result)
}

func TestCommandBuilder_WithVerbose(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		expected []string
	}{
		{"verbose without level", 0, []string{"backup", "--verbose"}},
		{"verbose level 1", 1, []string{"backup", "--verbose=1"}},
		{"verbose level 2", 2, []string{"backup", "--verbose=2"}},
		{"verbose level 3", 3, []string{"backup", "--verbose=3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithVerbose(tt.level)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected []string
	}{
		{"with host", "myhost", []string{"backup", "--host", "myhost"}},
		{"empty host", "", []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithHost(tt.host)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected []string
	}{
		{"with tag", "daily", []string{"backup", "--tag", "daily"}},
		{"empty tag", "", []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithTag(tt.tag)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected []string
	}{
		{"multiple tags", []string{"daily", "app1"}, []string{"backup", "--tag", "daily", "--tag", "app1"}},
		{"single tag", []string{"daily"}, []string{"backup", "--tag", "daily"}},
		{"empty tags", []string{}, []string{"backup"}},
		{"nil tags", nil, []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithTags(tt.tags)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithExclude(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{"with pattern", "*.tmp", []string{"backup", "--exclude", "*.tmp"}},
		{"empty pattern", "", []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithExclude(tt.pattern)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithExcludes(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		expected []string
	}{
		{"multiple patterns", []string{"*.tmp", "*.log"}, []string{"backup", "--exclude", "*.tmp", "--exclude", "*.log"}},
		{"single pattern", []string{"*.tmp"}, []string{"backup", "--exclude", "*.tmp"}},
		{"empty patterns", []string{}, []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithExcludes(tt.patterns)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithInclude(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{"with pattern", "/data", []string{"restore", "--include", "/data"}},
		{"empty pattern", "", []string{"restore"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("restore").WithInclude(tt.pattern)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithIncludes(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		expected []string
	}{
		{"multiple patterns", []string{"/data", "/config"}, []string{"restore", "--include", "/data", "--include", "/config"}},
		{"single pattern", []string{"/data"}, []string{"restore", "--include", "/data"}},
		{"empty patterns", []string{}, []string{"restore"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("restore").WithIncludes(tt.patterns)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected []string
	}{
		{"with target", "/restore", []string{"restore", "--target", "/restore"}},
		{"empty target", "", []string{"restore"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("restore").WithTarget(tt.target)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithKeepLast(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{"keep last 10", 10, []string{"forget", "--keep-last", "10"}},
		{"keep last 0", 0, []string{"forget"}},
		{"negative value", -1, []string{"forget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("forget").WithKeepLast(tt.n)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithKeepHourly(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{"keep hourly 24", 24, []string{"forget", "--keep-hourly", "24"}},
		{"keep hourly 0", 0, []string{"forget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("forget").WithKeepHourly(tt.n)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithKeepDaily(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{"keep daily 7", 7, []string{"forget", "--keep-daily", "7"}},
		{"keep daily 0", 0, []string{"forget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("forget").WithKeepDaily(tt.n)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithKeepWeekly(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{"keep weekly 4", 4, []string{"forget", "--keep-weekly", "4"}},
		{"keep weekly 0", 0, []string{"forget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("forget").WithKeepWeekly(tt.n)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithKeepMonthly(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{"keep monthly 12", 12, []string{"forget", "--keep-monthly", "12"}},
		{"keep monthly 0", 0, []string{"forget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("forget").WithKeepMonthly(tt.n)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithKeepYearly(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected []string
	}{
		{"keep yearly 3", 3, []string{"forget", "--keep-yearly", "3"}},
		{"keep yearly 0", 0, []string{"forget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("forget").WithKeepYearly(tt.n)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithGroupBy(t *testing.T) {
	tests := []struct {
		name     string
		groupBy  string
		expected []string
	}{
		{"group by host,tags", "host,tags", []string{"forget", "--group-by", "host,tags"}},
		{"group by host", "host", []string{"forget", "--group-by", "host"}},
		{"empty group by", "", []string{"forget"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("forget").WithGroupBy(tt.groupBy)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithPrune(t *testing.T) {
	cmd := NewCommand("forget").WithPrune()
	result := cmd.Build()

	expected := []string{"forget", "--prune"}
	assertArgs(t, expected, result)
}

func TestCommandBuilder_WithDryRun(t *testing.T) {
	cmd := NewCommand("forget").WithDryRun()
	result := cmd.Build()

	expected := []string{"forget", "--dry-run"}
	assertArgs(t, expected, result)
}

func TestCommandBuilder_WithPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{"with path", "/backup/data", []string{"backup", "/backup/data"}},
		{"empty path", "", []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithPath(tt.path)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithPaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected []string
	}{
		{"multiple paths", []string{"/data", "/config"}, []string{"backup", "/data", "/config"}},
		{"single path", []string{"/data"}, []string{"backup", "/data"}},
		{"empty paths", []string{}, []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithPaths(tt.paths)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithArg(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		expected []string
	}{
		{"with arg", "--verify", []string{"restore", "--verify"}},
		{"snapshot id", "abc12345", []string{"restore", "abc12345"}},
		{"empty arg", "", []string{"restore"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("restore").WithArg(tt.arg)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{"multiple args", []string{"--verbose", "--dry-run"}, []string{"backup", "--verbose", "--dry-run"}},
		{"single arg", []string{"--verbose"}, []string{"backup", "--verbose"}},
		{"empty args", []string{}, []string{"backup"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("backup").WithArgs(tt.args)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_WithMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected []string
	}{
		{"restore-size mode", "restore-size", []string{"stats", "--mode", "restore-size"}},
		{"raw-data mode", "raw-data", []string{"stats", "--mode", "raw-data"}},
		{"empty mode", "", []string{"stats"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand("stats").WithMode(tt.mode)
			result := cmd.Build()
			assertArgs(t, tt.expected, result)
		})
	}
}

func TestCommandBuilder_String(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected string
	}{
		{"backup command", "backup", "restic backup"},
		{"restore command", "restore", "restic restore"},
		{"init command", "init", "restic init"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand(tt.cmd)
			if cmd.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, cmd.String())
			}
		})
	}
}

func TestCommandBuilder_Chaining(t *testing.T) {
	// Test complex command building with chaining
	cmd := NewCommand("backup").
		WithJSON().
		WithHost("myhost").
		WithTags([]string{"daily", "app1"}).
		WithExcludes([]string{"*.tmp", "*.log"}).
		WithPaths([]string{"/data", "/config"})

	result := cmd.Build()

	expected := []string{
		"backup",
		"--json",
		"--host", "myhost",
		"--tag", "daily",
		"--tag", "app1",
		"--exclude", "*.tmp",
		"--exclude", "*.log",
		"/data",
		"/config",
	}

	assertArgs(t, expected, result)
}

func TestCommandBuilder_ForgetCommand(t *testing.T) {
	// Test forget command with all retention options
	cmd := NewCommand("forget").
		WithJSON().
		WithHost("myhost").
		WithTags([]string{"daily"}).
		WithKeepLast(10).
		WithKeepHourly(24).
		WithKeepDaily(7).
		WithKeepWeekly(4).
		WithKeepMonthly(12).
		WithKeepYearly(3).
		WithGroupBy("host,tags").
		WithPrune().
		WithDryRun()

	result := cmd.Build()

	expected := []string{
		"forget",
		"--json",
		"--host", "myhost",
		"--tag", "daily",
		"--keep-last", "10",
		"--keep-hourly", "24",
		"--keep-daily", "7",
		"--keep-weekly", "4",
		"--keep-monthly", "12",
		"--keep-yearly", "3",
		"--group-by", "host,tags",
		"--prune",
		"--dry-run",
	}

	assertArgs(t, expected, result)
}

func TestCommandBuilder_RestoreCommand(t *testing.T) {
	// Test restore command with typical options
	cmd := NewCommand("restore").
		WithTarget("/restore").
		WithIncludes([]string{"/data", "/config"}).
		WithExcludes([]string{"*.tmp"}).
		WithArg("abc12345").
		WithArg("--verify")

	result := cmd.Build()

	expected := []string{
		"restore",
		"--target", "/restore",
		"--include", "/data",
		"--include", "/config",
		"--exclude", "*.tmp",
		"abc12345",
		"--verify",
	}

	assertArgs(t, expected, result)
}

// assertArgs is a helper function to compare argument slices
func assertArgs(t *testing.T, expected, actual []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("expected %d args, got %d\nexpected: %v\nactual:   %v", len(expected), len(actual), expected, actual)
		return
	}
	for i, arg := range expected {
		if actual[i] != arg {
			t.Errorf("arg %d: expected %q, got %q\nexpected: %v\nactual:   %v", i, arg, actual[i], expected, actual)
			return
		}
	}
}
