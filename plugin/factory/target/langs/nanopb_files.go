package langs

// nanopb_files.go picks which protos nanopb compiles to C.
//
// Not every import: annotation protos (google/api/*, descriptor.proto) exist only
// to declare options, and nanopb cannot build their extensions — their field
// numbers overflow its default descriptor width. Firmware needs the requested
// files plus whatever their fields' types live in, and nothing else.

import (
	"google.golang.org/protobuf/types/descriptorpb"
)

// nanopbFiles returns the files to generate — the requested ones and those their
// fields' types come from — and the imports to drop from the #include list.
func nanopbFiles(requested []string, all []*descriptorpb.FileDescriptorProto) (generate, exclude []string) {
	byName := map[string]*descriptorpb.FileDescriptorProto{}
	owner := map[string]string{} // ".pkg.Type" -> file declaring it
	for _, f := range all {
		byName[f.GetName()] = f
		prefix := "." + f.GetPackage()
		if f.GetPackage() == "" {
			prefix = ""
		}
		for _, e := range f.GetEnumType() {
			owner[prefix+"."+e.GetName()] = f.GetName()
		}
		var walk func(scope string, msgs []*descriptorpb.DescriptorProto)
		walk = func(scope string, msgs []*descriptorpb.DescriptorProto) {
			for _, m := range msgs {
				name := scope + "." + m.GetName()
				owner[name] = f.GetName()
				for _, e := range m.GetEnumType() {
					owner[name+"."+e.GetName()] = f.GetName()
				}
				walk(name, m.GetNestedType())
			}
		}
		walk(prefix, f.GetMessageType())
	}

	need := map[string]bool{}
	queue := append([]string(nil), requested...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		f, ok := byName[name]
		if !ok || need[name] {
			continue
		}
		need[name] = true
		generate = append(generate, name)

		var walk func(msgs []*descriptorpb.DescriptorProto)
		walk = func(msgs []*descriptorpb.DescriptorProto) {
			for _, m := range msgs {
				for _, field := range m.GetField() {
					if dep, ok := owner[field.GetTypeName()]; ok && !need[dep] {
						queue = append(queue, dep)
					}
				}
				walk(m.GetNestedType())
			}
		}
		walk(f.GetMessageType())
	}

	seen := map[string]bool{}
	for _, name := range generate {
		for _, dep := range byName[name].GetDependency() {
			if !need[dep] && !seen[dep] {
				seen[dep] = true
				exclude = append(exclude, dep)
			}
		}
	}
	return generate, exclude
}
