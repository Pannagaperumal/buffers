package nanopb

import (
	"testing"

	"github.com/the-protobuf-project/protokit/buffers"
)

// TestOptions pins each annotation to the nanopb option it becomes, and which
// shapes are reported as unbounded.
func TestOptions(t *testing.T) {
	str := &buffers.Field{Kind: buffers.KindString}
	for _, c := range []struct {
		name  string
		field buffers.Field
		want  string
		diags int
	}{
		{"bounded string", buffers.Field{Kind: buffers.KindString, MaxLen: 32}, "max_length:32", 0},
		{"unbounded string", buffers.Field{Kind: buffers.KindString}, "", 1},
		{"bounded bytes", buffers.Field{Kind: buffers.KindBytes, MaxLen: 64}, "max_size:64", 0},
		{"bounded list", buffers.Field{Kind: buffers.KindFloat, Repeated: true, MaxLen: 8}, "max_count:8", 0},
		{"fixed list", buffers.Field{Kind: buffers.KindFloat, Repeated: true, FixedLen: 3}, "max_count:3 fixed_count:true", 0},
		{"unbounded list", buffers.Field{Kind: buffers.KindFloat, Repeated: true}, "", 1},
		{"list of unbounded strings", buffers.Field{Kind: buffers.KindString, Repeated: true, MaxLen: 4}, "max_count:4", 1},
		{"map of strings", buffers.Field{Kind: buffers.KindMap, MapKey: str, MapValue: str, MaxLen: 4}, "max_count:4", 1},
		{"scalar", buffers.Field{Kind: buffers.KindInt32}, "", 0},
		{"skipped", buffers.Field{Kind: buffers.KindString, Skip: true}, "type:FT_IGNORE", 0},
		{"other target only", buffers.Field{Kind: buffers.KindString, Targets: []string{"ros"}}, "type:FT_IGNORE", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := &run{}
			if got := r.options(&c.field); got != c.want {
				t.Errorf("options = %q, want %q", got, c.want)
			}
			if len(r.diags) != c.diags {
				t.Errorf("%d diagnostics, want %d: %v", len(r.diags), c.diags, r.diags)
			}
		})
	}
}
