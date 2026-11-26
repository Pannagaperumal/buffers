package langs

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// TestNanopbFiles keeps imports whose types a field uses and drops the
// annotation-only ones nanopb cannot compile.
func TestNanopbFiles(t *testing.T) {
	file := func(name, pkg string, deps []string, msgs ...*descriptorpb.DescriptorProto) *descriptorpb.FileDescriptorProto {
		return &descriptorpb.FileDescriptorProto{Name: proto.String(name), Package: proto.String(pkg), Dependency: deps, MessageType: msgs}
	}
	msg := func(name string, fieldTypes ...string) *descriptorpb.DescriptorProto {
		m := &descriptorpb.DescriptorProto{Name: proto.String(name)}
		for _, ft := range fieldTypes {
			m.Field = append(m.Field, &descriptorpb.FieldDescriptorProto{TypeName: proto.String(ft)})
		}
		return m
	}

	all := []*descriptorpb.FileDescriptorProto{
		file("google/protobuf/descriptor.proto", "google.protobuf", nil, msg("FieldOptions")),
		file("google/api/field_behavior.proto", "google.api", []string{"google/protobuf/descriptor.proto"}),
		file("google/protobuf/timestamp.proto", "google.protobuf", nil, msg("Timestamp")),
		file("geo.proto", "app", nil, msg("Point")),
		file("app.proto", "app", []string{"google/api/field_behavior.proto", "google/protobuf/timestamp.proto", "geo.proto"},
			msg("Robot", ".google.protobuf.Timestamp", ".app.Point")),
	}

	gen, exclude := nanopbFiles([]string{"app.proto"}, all)
	if want := []string{"app.proto", "google/protobuf/timestamp.proto", "geo.proto"}; !reflect.DeepEqual(gen, want) {
		t.Errorf("generate = %v, want %v", gen, want)
	}
	if want := []string{"google/api/field_behavior.proto"}; !reflect.DeepEqual(exclude, want) {
		t.Errorf("exclude = %v, want %v", exclude, want)
	}
}
