package langs

// nanopb.go runs nanopb over the descriptors a run already loaded.
//
// nanopb is unlike the other three toolchains: it compiles the .proto, not the
// schema buffers emits, and reads the emitted .options as side input. Its
// protoc-gen-nanopb is a protoc plugin, so it is handed the CodeGeneratorRequest
// directly — protoc itself is not needed, only `pip install nanopb`.

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// nanopbPlugin is the binary nanopb installs for protoc.
const nanopbPlugin = "protoc-gen-nanopb"

// Nanopb generates C for every file in req, reading .options from optionsDir,
// which must be absolute since the plugin resolves it against its own cwd.
func Nanopb(req *pluginpb.CodeGeneratorRequest, optionsDir string) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	if _, err := exec.LookPath(nanopbPlugin); err != nil {
		return nil, fmt.Errorf("%s is not on PATH, and the nanopb target needs it to produce c.\n"+
			"    install: pip install nanopb", nanopbPlugin)
	}

	in := proto.Clone(req).(*pluginpb.CodeGeneratorRequest)
	generate, exclude := nanopbFiles(req.GetFileToGenerate(), req.GetProtoFile())
	in.FileToGenerate = generate
	// Comma-separated, protoc style. shortcut: a comma in optionsDir would split
	// it; switch to quoting if that ever comes up.
	params := []string{"-I" + optionsDir}
	for _, dep := range exclude {
		params = append(params, "-x"+dep)
	}
	in.Parameter = proto.String(strings.Join(params, ","))
	data, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(nanopbPlugin)
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", nanopbPlugin, err, stderr.String())
	}

	var resp pluginpb.CodeGeneratorResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("%s: unreadable response: %w", nanopbPlugin, err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("%s: %s", nanopbPlugin, resp.GetError())
	}
	return resp.GetFile(), nil
}
