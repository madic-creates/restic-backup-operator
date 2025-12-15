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
	"fmt"
	"strconv"
)

// CommandBuilder builds restic command arguments.
type CommandBuilder struct {
	command string
	args    []string
}

// NewCommand creates a new command builder.
func NewCommand(cmd string) *CommandBuilder {
	return &CommandBuilder{
		command: cmd,
		args:    []string{cmd},
	}
}

// WithJSON adds the --json flag.
func (b *CommandBuilder) WithJSON() *CommandBuilder {
	b.args = append(b.args, "--json")
	return b
}

// WithQuiet adds the --quiet flag.
func (b *CommandBuilder) WithQuiet() *CommandBuilder {
	b.args = append(b.args, "--quiet")
	return b
}

// WithVerbose adds the --verbose flag with optional level.
func (b *CommandBuilder) WithVerbose(level int) *CommandBuilder {
	if level > 0 {
		b.args = append(b.args, fmt.Sprintf("--verbose=%d", level))
	} else {
		b.args = append(b.args, "--verbose")
	}
	return b
}

// WithHost adds the --host flag.
func (b *CommandBuilder) WithHost(host string) *CommandBuilder {
	if host != "" {
		b.args = append(b.args, "--host", host)
	}
	return b
}

// WithTag adds a --tag flag.
func (b *CommandBuilder) WithTag(tag string) *CommandBuilder {
	if tag != "" {
		b.args = append(b.args, "--tag", tag)
	}
	return b
}

// WithTags adds multiple --tag flags.
func (b *CommandBuilder) WithTags(tags []string) *CommandBuilder {
	for _, tag := range tags {
		b.WithTag(tag)
	}
	return b
}

// WithExclude adds an --exclude flag.
func (b *CommandBuilder) WithExclude(pattern string) *CommandBuilder {
	if pattern != "" {
		b.args = append(b.args, "--exclude", pattern)
	}
	return b
}

// WithExcludes adds multiple --exclude flags.
func (b *CommandBuilder) WithExcludes(patterns []string) *CommandBuilder {
	for _, pattern := range patterns {
		b.WithExclude(pattern)
	}
	return b
}

// WithInclude adds an --include flag.
func (b *CommandBuilder) WithInclude(pattern string) *CommandBuilder {
	if pattern != "" {
		b.args = append(b.args, "--include", pattern)
	}
	return b
}

// WithIncludes adds multiple --include flags.
func (b *CommandBuilder) WithIncludes(patterns []string) *CommandBuilder {
	for _, pattern := range patterns {
		b.WithInclude(pattern)
	}
	return b
}

// WithTarget adds the --target flag.
func (b *CommandBuilder) WithTarget(target string) *CommandBuilder {
	if target != "" {
		b.args = append(b.args, "--target", target)
	}
	return b
}

// WithKeepLast adds the --keep-last flag.
func (b *CommandBuilder) WithKeepLast(n int) *CommandBuilder {
	if n > 0 {
		b.args = append(b.args, "--keep-last", strconv.Itoa(n))
	}
	return b
}

// WithKeepHourly adds the --keep-hourly flag.
func (b *CommandBuilder) WithKeepHourly(n int) *CommandBuilder {
	if n > 0 {
		b.args = append(b.args, "--keep-hourly", strconv.Itoa(n))
	}
	return b
}

// WithKeepDaily adds the --keep-daily flag.
func (b *CommandBuilder) WithKeepDaily(n int) *CommandBuilder {
	if n > 0 {
		b.args = append(b.args, "--keep-daily", strconv.Itoa(n))
	}
	return b
}

// WithKeepWeekly adds the --keep-weekly flag.
func (b *CommandBuilder) WithKeepWeekly(n int) *CommandBuilder {
	if n > 0 {
		b.args = append(b.args, "--keep-weekly", strconv.Itoa(n))
	}
	return b
}

// WithKeepMonthly adds the --keep-monthly flag.
func (b *CommandBuilder) WithKeepMonthly(n int) *CommandBuilder {
	if n > 0 {
		b.args = append(b.args, "--keep-monthly", strconv.Itoa(n))
	}
	return b
}

// WithKeepYearly adds the --keep-yearly flag.
func (b *CommandBuilder) WithKeepYearly(n int) *CommandBuilder {
	if n > 0 {
		b.args = append(b.args, "--keep-yearly", strconv.Itoa(n))
	}
	return b
}

// WithGroupBy adds the --group-by flag.
func (b *CommandBuilder) WithGroupBy(groupBy string) *CommandBuilder {
	if groupBy != "" {
		b.args = append(b.args, "--group-by", groupBy)
	}
	return b
}

// WithPrune adds the --prune flag.
func (b *CommandBuilder) WithPrune() *CommandBuilder {
	b.args = append(b.args, "--prune")
	return b
}

// WithDryRun adds the --dry-run flag.
func (b *CommandBuilder) WithDryRun() *CommandBuilder {
	b.args = append(b.args, "--dry-run")
	return b
}

// WithPath adds a path argument.
func (b *CommandBuilder) WithPath(path string) *CommandBuilder {
	if path != "" {
		b.args = append(b.args, path)
	}
	return b
}

// WithPaths adds multiple path arguments.
func (b *CommandBuilder) WithPaths(paths []string) *CommandBuilder {
	b.args = append(b.args, paths...)
	return b
}

// WithArg adds a custom argument.
func (b *CommandBuilder) WithArg(arg string) *CommandBuilder {
	if arg != "" {
		b.args = append(b.args, arg)
	}
	return b
}

// WithArgs adds multiple custom arguments.
func (b *CommandBuilder) WithArgs(args []string) *CommandBuilder {
	b.args = append(b.args, args...)
	return b
}

// WithMode adds the --mode flag (for stats).
func (b *CommandBuilder) WithMode(mode string) *CommandBuilder {
	if mode != "" {
		b.args = append(b.args, "--mode", mode)
	}
	return b
}

// Build returns the final arguments slice.
func (b *CommandBuilder) Build() []string {
	return b.args
}

// String returns the command as a string for logging.
func (b *CommandBuilder) String() string {
	return fmt.Sprintf("restic %s", b.command)
}
