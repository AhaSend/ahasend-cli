package cmdutil

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// The alias functions add hidden, deprecated names of a flag, which set the
// flag, so the command reads only the flag. Giving a flag under two of its
// names, such as --enabled and its alias --disable, is a validation error
// that names them in the order given, after the deprecation notice of each
// deprecated name given.

// AddFlagAlias adds old as a hidden, deprecated name of the flag name. Each
// value given to old is set on name, so the command reads only name. The
// alias of a bool flag is a switch, as addSwitchAlias describes.
func AddFlagAlias(cmd *cobra.Command, old, name string) {
	if cmd.Flags().Lookup(name).Value.Type() == "bool" {
		addSwitchAlias(cmd, old, name, fmt.Sprintf("use --%s instead", name), "true",
			fmt.Sprintf("give --%s", name), fmt.Sprintf("leave it out, or give --%s=false", name))
		return
	}
	addAlias(cmd, old, name, fmt.Sprintf("use --%s instead", name), func(value string) (string, error) { return value, nil })
}

// AddNegatedFlagAlias adds old as a hidden, deprecated switch that sets the
// bool flag name to false: --old is --name=false.
func AddNegatedFlagAlias(cmd *cobra.Command, old, name string) {
	addSwitchAlias(cmd, old, name, fmt.Sprintf("use --%s=false instead", name), "false",
		fmt.Sprintf("give --%s=false", name), fmt.Sprintf("leave it out, or give --%s", name))
}

// AddValueAlias adds old as a hidden, deprecated switch that sets the flag
// name to value: --old is --name value.
func AddValueAlias(cmd *cobra.Command, old, name, value string) {
	addSwitchAlias(cmd, old, name, fmt.Sprintf("use --%s %s instead", name, value), value,
		fmt.Sprintf("give --%s %s", name, value), "leave it out")
}

// AddInteractiveAlias adds --interactive as a hidden, deprecated switch that
// sets --non-interactive to false, for a command that had --interactive
// before --non-interactive replaced it.
func AddInteractiveAlias(cmd *cobra.Command) {
	addSwitchAlias(cmd, "interactive", "non-interactive", "prompting is the default; use --non-interactive to turn it off", "false",
		"leave it out, as prompting is the default",
		"leave it out, as prompting is the default, or give --non-interactive to turn prompting off")
}

// noValue is the value pflag passes a switch given without a value. No
// command line can hold a NUL byte, so no value a user gives is noValue.
const noValue = "\x00"

// addSwitchAlias adds old as a hidden, deprecated switch: a flag that takes
// no value and sets the flag name to set. A value given to it, such as
// --old=false, is a validation error rather than a change of meaning, since
// v0.2.0 read --old=false as not giving the switch. The error names what to
// give in its place: whenTrue for a true value, such as "give --enabled",
// and whenFalse for a false one, such as "leave it out, or give
// --enabled=false"; a value that is neither reads as true.
func addSwitchAlias(cmd *cobra.Command, old, name, deprecation, set, whenTrue, whenFalse string) {
	addAlias(cmd, old, name, deprecation, func(value string) (string, error) {
		if value == noValue {
			return set, nil
		}
		instead := whenTrue
		if given, err := strconv.ParseBool(value); err == nil && !given {
			instead = whenFalse
		}
		return "", errors.NewValidationError(fmt.Sprintf("--%s takes no value; %s", old, instead), nil)
	})
	cmd.Flags().Lookup(old).NoOptDefVal = noValue
}

func addAlias(cmd *cobra.Command, old, name, deprecation string, convert func(string) (string, error)) {
	flags := cmd.Flags()
	aliasTarget := aliasTargetOf(cmd, name)
	target := flags.Lookup(name)
	flags.AddFlag(&pflag.Flag{
		Name:        old,
		Usage:       target.Usage,
		Value:       &aliasValue{flags: flags, name: name, old: old, target: aliasTarget, convert: convert},
		DefValue:    target.DefValue,
		NoOptDefVal: target.NoOptDefVal,
	})
	_ = flags.MarkDeprecated(old, deprecation)
}

// aliasTarget is the value of a flag that has aliases. It records the name
// that set the flag, so that setting it under another name fails.
type aliasTarget struct {
	pflag.Value
	cmd    *cobra.Command
	name   string // the flag's own name
	setBy  string // the name that set the flag, or ""
	setter string // the alias setting the flag now, or ""
}

// aliasTargetOf returns the aliasTarget of the flag name of cmd, which it
// installs as the flag's value the first time.
func aliasTargetOf(cmd *cobra.Command, name string) *aliasTarget {
	flag := cmd.Flags().Lookup(name)
	if target, ok := flag.Value.(*aliasTarget); ok {
		return target
	}
	target := &aliasTarget{Value: flag.Value, cmd: cmd, name: name}
	flag.Value = target
	// Help leaves out a default that is the zero value, but knows "[]" as
	// the zero value of a list flag only by the value's type, which the
	// wrapper hides; "" is the zero value of every type.
	if flag.DefValue == "[]" {
		flag.DefValue = ""
	}
	return target
}

// TypedName returns the name the user gave the flag name under: a deprecated
// alias of it, or name itself.
func TypedName(cmd *cobra.Command, name string) string {
	if target, ok := cmd.Flags().Lookup(name).Value.(*aliasTarget); ok && target.setBy != "" {
		return target.setBy
	}
	return name
}

func (t *aliasTarget) Set(value string) error {
	by := t.name
	if t.setter != "" {
		by = t.setter
	}
	if t.setBy != "" && t.setBy != by {
		t.warnIfDeprecated(t.setBy)
		t.warnIfDeprecated(by)
		return errors.NewValidationError(fmt.Sprintf("--%s and --%s cannot be used together", t.setBy, by), nil)
	}
	t.setBy = by
	if repeatable, ok := t.Value.(*repeatableValue); ok {
		return repeatable.setAs(by, value)
	}
	return t.Value.Set(value)
}

// warnIfDeprecated writes the deprecation notice of the flag name, when it is
// deprecated, as pflag words it. A flag that fails to parse stops Cobra from
// printing the notices pflag wrote, so a conflict prints them itself.
func (t *aliasTarget) warnIfDeprecated(name string) {
	if flag := t.cmd.Flags().Lookup(name); flag.Deprecated != "" {
		fmt.Fprintf(t.cmd.ErrOrStderr(), "Flag --%s has been deprecated, %s\n", name, flag.Deprecated)
	}
}

// setAs sets the flag to value on behalf of the alias old.
func (t *aliasTarget) setAs(flags *pflag.FlagSet, old, value string) error {
	t.setter = old
	defer func() { t.setter = "" }()
	return flags.Set(t.name, value)
}

// aliasValue sets every value it is given on the flag name, which marks that
// flag as changed.
type aliasValue struct {
	flags   *pflag.FlagSet
	name    string
	old     string
	target  *aliasTarget
	convert func(string) (string, error)
}

func (v *aliasValue) Set(value string) error {
	converted, err := v.convert(value)
	if err != nil {
		return err
	}
	return v.target.setAs(v.flags, v.old, converted)
}

func (v *aliasValue) String() string { return v.flags.Lookup(v.name).Value.String() }

func (v *aliasValue) Type() string { return v.flags.Lookup(v.name).Value.Type() }
