// Package nanopb renders nanopb generator options: one .options file per proto,
// for C on microcontrollers — ESP32, STM32, nRF, RP2040.
//
// # Why this target emits no schema
//
// Like wire, nanopb consumes .proto directly, and the device speaks plain
// protobuf on the wire — so field numbers are the slots and there is no ordinal
// mapping to pin. What nanopb needs that a .proto does not carry is a size for
// every string, bytes and repeated field. Without one it falls back to
// pb_callback_t, which means hand-written decode callbacks and usually the heap;
// with one it emits a fixed-size struct member, which is what firmware wants.
//
// (buffers.v1.field).max_len and fixed_len already carry those sizes for ROS, so
// this target turns them into nanopb options and reports every field left
// unbounded.
//
// # What it emits
//
//	<proto path>.options   e.g. sensors/v1/sensors.options
//
// The path mirrors the proto, which is where nanopb_generator looks for it when
// the output directory is passed as an options path (`-I <out>`).
package nanopb

import (
	"fmt"
	"strings"

	"github.com/the-protobuf-project/protokit/buffers"
	"github.com/the-protobuf-project/protokit/factory"

	"github.com/the-protobuf-project/buffers/plugin/factory/coreir"
	"github.com/the-protobuf-project/buffers/plugin/factory/provenance"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/emit"
)

// Target renders nanopb .options files.
type Target struct {
	// sink receives each finished file, relative to the output directory.
	sink emit.Sink
	// info is the provenance stamped into every banner.
	info provenance.Info
}

// New returns a nanopb target writing through the given sink.
func New(sink emit.Sink, info provenance.Info) *Target {
	info.Target = "nanopb"
	// nanopb uses proto field numbers as-is, so buffers.lock says nothing about it.
	info.NoLedger = true
	return &Target{sink: sink, info: info}
}

// Name identifies the target in the registry and in `target=` options.
func (t *Target) Name() string { return "nanopb" }

// Languages lists what nanopb generates. The consuming build (ESP-IDF, CMake,
// PlatformIO) runs nanopb_generator; the CLI does not.
func (t *Target) Languages() []string { return []string{"schema", "c"} }

// Generate renders one .options file per proto file.
func (t *Target) Generate(_ factory.Ctx, m *coreir.Model, lang string) error {
	if lang != "" && lang != "schema" && lang != "c" {
		return fmt.Errorf("nanopb cannot emit %q; nanopb produces: c", lang)
	}
	r := &run{Target: t}
	for _, f := range m.Schema.Files {
		if err := r.file(f); err != nil {
			return err
		}
	}
	m.Schema.Diags = append(m.Schema.Diags, r.diags...)
	return nil
}

// run is one Generate call's state.
type run struct {
	*Target
	// diags accumulates unbounded fields.
	diags []buffers.Diagnostic
}

// file renders the .options for one proto file.
func (r *run) file(f *buffers.File) error {
	info := r.info
	info.Source = f.Path
	var b emit.Buf
	b.Raw(provenance.Render(provenance.Hash, info))
	b.Line("")

	var walk func(msgs []*buffers.Message)
	walk = func(msgs []*buffers.Message) {
		for _, m := range msgs {
			switch {
			case m.IsMapEntry:
				// Bounded through the map field itself.
			case m.Skip || !allows(m.Targets):
				b.Linef("%s skip_message:true", m.Node)
			default:
				for _, field := range m.Fields {
					if opts := r.options(field); opts != "" {
						b.Linef("%s %s", field.Node, opts)
					}
				}
			}
			walk(m.Nested)
		}
	}
	walk(f.Messages)

	return r.sink(strings.TrimSuffix(f.Path, ".proto")+".options", b.Bytes())
}

// options returns the nanopb options for one field, or "" when it needs none.
func (r *run) options(f *buffers.Field) string {
	if f.Skip || !allows(f.Targets) {
		return "type:FT_IGNORE"
	}
	switch {
	case f.Repeated || f.Kind == buffers.KindMap:
		if f.FixedLen > 0 {
			r.element(f)
			return fmt.Sprintf("max_count:%d fixed_count:true", f.FixedLen)
		}
		if f.MaxLen > 0 {
			r.element(f)
			return fmt.Sprintf("max_count:%d", f.MaxLen)
		}
		r.unbounded(f, "repeated field has no max_count")
	case f.Kind == buffers.KindString:
		if f.MaxLen > 0 {
			return fmt.Sprintf("max_length:%d", f.MaxLen)
		}
		r.unbounded(f, "string has no max_length")
	case f.Kind == buffers.KindBytes:
		if f.MaxLen > 0 {
			return fmt.Sprintf("max_size:%d", f.MaxLen)
		}
		r.unbounded(f, "bytes has no max_size")
	}
	return ""
}

// element reports a bounded list whose elements are themselves unbounded, which
// max_len cannot express: it carries one number, and here it went to the count.
func (r *run) element(f *buffers.Field) {
	if f.Kind == buffers.KindMap {
		if isVariable(f.MapKey) || isVariable(f.MapValue) {
			r.collect(f, "map entries carry an unbounded string or bytes key or value, so nanopb decodes them through pb_callback_t",
				"replace the map with a repeated message whose string fields carry (buffers.v1.field).max_len")
		}
		return
	}
	if isVariable(f) {
		r.collect(f, "max_len bounds the element count, but each string or bytes element is still unbounded, so nanopb decodes the field through pb_callback_t",
			"wrap the element in a message whose string field carries its own (buffers.v1.field).max_len")
	}
}

// unbounded reports a field nanopb will decode through a callback.
func (r *run) unbounded(f *buffers.Field, what string) {
	r.collect(f, what+"; nanopb decodes it through pb_callback_t instead of a fixed-size struct member",
		"add (buffers.v1.field) = {max_len: N}")
}

// collect records a target diagnostic.
func (r *run) collect(f *buffers.Field, msg, hint string) {
	r.diags = append(r.diags, buffers.Diagnostic{Rule: buffers.RuleTarget, Node: f.Node, Message: msg, Hint: hint})
}

// isVariable reports whether a value has no size of its own.
func isVariable(f *buffers.Field) bool {
	return f != nil && (f.Kind == buffers.KindString || f.Kind == buffers.KindBytes)
}

// allows reports whether a target allow-list admits this target. An empty list
// admits everything.
func allows(list []string) bool {
	if len(list) == 0 {
		return true
	}
	for _, t := range list {
		if t == "nanopb" {
			return true
		}
	}
	return false
}
