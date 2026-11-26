package main

// nanopb.go compiles a nanopb entry. It is apart from compile.go because nanopb
// reads the protos rather than the schema this tool emits, so none of the
// schema-file grouping there applies.

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/pluginpb"

	"github.com/the-protobuf-project/buffers/plugin/factory/config"
	"github.com/the-protobuf-project/buffers/plugin/factory/target/langs"
)

// compileNanopb writes nanopb's .pb.c/.pb.h for an entry that asked for c.
func compileNanopb(cmd *cobra.Command, cfg *config.Config, e config.Entry, languages []string, req *pluginpb.CodeGeneratorRequest) error {
	for _, l := range languages {
		if l == "schema" {
			continue
		}
		if l != "c" {
			return fail("nanopb produces c, not %q", l)
		}

		optionsDir, err := filepath.Abs(cfg.OutDir(e))
		if err != nil {
			return err
		}
		files, err := langs.Nanopb(req, optionsDir)
		if err != nil {
			return err
		}

		out := cfg.LangDir(e, l)
		sink, _ := diskSink(out, false, cmd)
		for _, f := range files {
			if err := sink(f.GetName(), []byte(f.GetContent())); err != nil {
				return err
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-12s -> %s (%s, %d files)\n", e.Target, out, l, len(files))
	}
	return nil
}
