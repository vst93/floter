package terminalui

import "testing"

func TestSessionsEnablePromptIntegration(t *testing.T) {
	create, options := captureTerminal(t)
	a := New(newStore(), Actions{}, create)
	a.NewSession()
	if err := a.RunCommand([]string{"zsh", "-l"}, "/tmp", []string{"FLOTER_TEST=1"}); err != nil {
		t.Fatal(err)
	}
	if len(*options) != 2 {
		t.Fatalf("created %d terminals, want 2", len(*options))
	}
	for i, opts := range *options {
		if !opts.ShellIntegration {
			t.Errorf("session %d did not enable prompt integration", i)
		}
	}
	command := (*options)[1]
	if len(command.Command) != 2 || command.Command[0] != "zsh" || command.Command[1] != "-l" || command.Dir != "/tmp" {
		t.Fatalf("command launch changed: command=%v dir=%q", command.Command, command.Dir)
	}
	if len(command.Env) != 1 || command.Env[0] != "FLOTER_TEST=1" {
		t.Fatalf("command environment changed: %v", command.Env)
	}
}
