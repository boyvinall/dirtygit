package scanner

import (
	"reflect"
	"testing"
)

func TestConfigEditArgv(t *testing.T) {
	t.Parallel()

	t.Run("default is code plus path", func(t *testing.T) {
		t.Parallel()
		var c Config
		got, err := c.EditArgv("/projects/foo")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"code", "/projects/foo"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EditArgv() = %#v, want %#v", got, want)
		}
	})

	t.Run("append path when no placeholder", func(t *testing.T) {
		t.Parallel()
		var c Config
		c.Edit.Command = []string{"cursor", "--reuse-window"}
		got, err := c.EditArgv("/r")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"cursor", "--reuse-window", "/r"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EditArgv() = %#v, want %#v", got, want)
		}
	})

	t.Run("placeholder substitution", func(t *testing.T) {
		t.Parallel()
		var c Config
		c.Edit.Command = []string{"myedit", "-d", "{repo}/src"}
		got, err := c.EditArgv("/abs/repo")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"myedit", "-d", "/abs/repo/src"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EditArgv() = %#v, want %#v", got, want)
		}
	})

	t.Run("empty program errors", func(t *testing.T) {
		t.Parallel()
		var c Config
		c.Edit.Command = []string{""}
		_, err := c.EditArgv("/r")
		if err == nil {
			t.Fatal("expected error for empty program")
		}
	})
}

func TestBranchStatusHasLocalRemoteMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   BranchStatus
		want bool
	}{
		{
			name: "detached head",
			in: BranchStatus{
				Detached: true,
			},
			want: false,
		},
		{
			name: "no remotes",
			in: BranchStatus{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
				},
			},
			want: false,
		},
		{
			name: "matching local and remote",
			in: BranchStatus{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
					{Name: "origin", Exists: true, TipHash: "abc"},
				},
			},
			want: false,
		},
		{
			name: "local ahead of remote",
			in: BranchStatus{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "aaa"},
					{Name: "origin", Exists: true, TipHash: "bbb"},
				},
			},
			want: true,
		},
		{
			name: "remote branch missing",
			in: BranchStatus{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "aaa"},
					{Name: "origin", Exists: false},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.HasLocalRemoteMismatch(); got != tt.want {
				t.Fatalf("HasLocalRemoteMismatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalBranchRefHasTipMismatchAcrossRemotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lb   LocalBranchRef
		want bool
	}{
		{
			name: "empty locations",
			lb:   LocalBranchRef{Name: "main"},
			want: true,
		},
		{
			name: "no remotes in locations",
			lb: LocalBranchRef{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
				},
			},
			want: true,
		},
		{
			name: "all remotes match local",
			lb: LocalBranchRef{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
					{Name: "origin", Exists: true, TipHash: "abc"},
					{Name: "fork", Exists: true, TipHash: "abc"},
				},
			},
			want: false,
		},
		{
			name: "one remote differs",
			lb: LocalBranchRef{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
					{Name: "origin", Exists: true, TipHash: "abc"},
					{Name: "fork", Exists: true, TipHash: "def"},
				},
			},
			want: true,
		},
		{
			name: "remote ref missing",
			lb: LocalBranchRef{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
					{Name: "origin", Exists: true, TipHash: "abc"},
					{Name: "fork", Exists: false},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.lb.HasTipMismatchAcrossRemotes(); got != tt.want {
				t.Fatalf("HasTipMismatchAcrossRemotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalBranchRefIsLocalOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lb   LocalBranchRef
		want bool
	}{
		{
			name: "empty locations",
			lb:   LocalBranchRef{Name: "main"},
			want: false,
		},
		{
			name: "only local slot no remotes",
			lb: LocalBranchRef{
				Name: "feature",
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
				},
			},
			want: true,
		},
		{
			name: "remote has same name ref",
			lb: LocalBranchRef{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
					{Name: "origin", Exists: true, TipHash: "abc"},
				},
			},
			want: false,
		},
		{
			name: "all configured remotes missing ref",
			lb: LocalBranchRef{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
					{Name: "origin", Exists: false},
					{Name: "fork", Exists: false},
				},
			},
			want: true,
		},
		{
			name: "one of two remotes has ref",
			lb: LocalBranchRef{
				Locations: []BranchLocation{
					{Name: "local", Exists: true, TipHash: "abc"},
					{Name: "origin", Exists: false},
					{Name: "fork", Exists: true, TipHash: "def"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.lb.IsLocalOnly(); got != tt.want {
				t.Fatalf("IsLocalOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}
