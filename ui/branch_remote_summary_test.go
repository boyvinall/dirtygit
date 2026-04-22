package ui

import (
	"testing"

	"github.com/boyvinall/dirtygit/scanner"
)

func TestBranchRemoteSummaryFromLocations(t *testing.T) {
	t.Parallel()

	local := scanner.BranchLocation{Name: "local", Exists: true, TipHash: "aaa111"}

	tests := []struct {
		name string
		locs []scanner.BranchLocation
		want string
	}{
		{
			name: "empty",
			locs: nil,
			want: "-",
		},
		{
			name: "no local",
			locs: []scanner.BranchLocation{
				{Name: "origin", Exists: true, TipHash: "bbb"},
			},
			want: "-",
		},
		{
			name: "remote missing",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: false},
			},
			want: "origin: missing",
		},
		{
			name: "in sync",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: true, TipHash: "aaa111"},
			},
			want: "origin: ok",
		},
		{
			name: "remote ahead",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: true, TipHash: "bbb", Incoming: 3},
			},
			want: "origin +3",
		},
		{
			name: "remote behind",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: true, TipHash: "bbb", Outgoing: 5},
			},
			want: "origin -5",
		},
		{
			name: "diverged",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: true, TipHash: "bbb", Incoming: 2, Outgoing: 4},
			},
			want: "origin +2-4",
		},
		{
			name: "unrelated",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: true, TipHash: "bbb", HistoriesUnrelated: true},
			},
			want: "origin: differs",
		},
		{
			name: "tip mismatch no deltas",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: true, TipHash: "bbb"},
			},
			want: "origin: differs",
		},
		{
			name: "two remotes",
			locs: []scanner.BranchLocation{
				local,
				{Name: "origin", Exists: true, TipHash: "bbb", Incoming: 1},
				{Name: "fork", Exists: true, TipHash: "ccc", Outgoing: 2},
			},
			want: "origin +1, fork -2",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := branchRemoteSummaryFromLocations(tt.locs); got != tt.want {
				t.Fatalf("branchRemoteSummaryFromLocations() = %q, want %q", got, tt.want)
			}
		})
	}
}
