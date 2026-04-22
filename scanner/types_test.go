package scanner

import "testing"

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
