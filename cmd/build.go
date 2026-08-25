//go:build linux

package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cardinal/internal/builder"
)

func Build(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	tag := fs.String("t", "", "Image name and tag (e.g. myapp:latest)")
	dockerfile := fs.String("f", "", "Path to Dockerfile (default: <context>/Dockerfile)")
	noCache := fs.Bool("no-cache", false, "Do not use cache when building the image")
	var buildArgs builder.StringSlice
	fs.Var(&buildArgs, "build-arg", "Build-time variables")
	quiet := fs.Bool("quiet", false, "Suppress build output")
	cpu := fs.Float64("cpu", 0, "CPU cores (e.g. 0.5, 2)")
	memory := fs.Int("memory", 0, "Memory limit in bytes (e.g. 536870912 for 512MB)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing build options: %v\n", err)
		os.Exit(1)
	}

	if *tag == "" {
		fmt.Println("Usage: cardinal build -t <name>[:<tag>] [options] <context>")
		fmt.Println("  -t name:tag    Image name and tag (required)")
		fmt.Println("  -f Dockerfile  Path to Dockerfile (default: ./Dockerfile)")
		fmt.Println("  --no-cache     Disable layer caching")
		fmt.Println("  --build-arg K=V  Set build-time variables")
		os.Exit(1)
	}

	freeArgs := fs.Args()
	contextDir := "."
	if len(freeArgs) > 0 {
		contextDir = freeArgs[0]
	}

	// Parse tag into name:tag
	imgName := *tag
	imgTag := "latest"
	if i := strings.LastIndex(*tag, ":"); i > 0 {
		imgTag = (*tag)[i+1:]
		imgName = (*tag)[:i]
	}

	// Parse build args
	buildArgMap := make(map[string]string)
	for _, ba := range buildArgs {
		if parts := strings.SplitN(ba, "=", 2); len(parts) == 2 {
			buildArgMap[parts[0]] = parts[1]
		}
	}

	dfPath := *dockerfile
	if dfPath == "" {
		dfPath = filepath.Join(contextDir, "Dockerfile")
	}

	if _, err := os.Stat(contextDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: build context %s not found\n", contextDir)
		os.Exit(1)
	}

	if _, err := os.Stat(dfPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Dockerfile not found at %s\n", dfPath)
		os.Exit(1)
	}

	cfg := &builder.BuildConfig{
		ContextDir:  contextDir,
		Dockerfile:  dfPath,
		ImageName:   imgName,
		Tag:         imgTag,
		NoCache:     *noCache,
		BuildArgs:   buildArgMap,
		Quiet:       *quiet,
		CPUCount:    *cpu,
		MemoryLimit: int64(*memory),
	}

	_, err := builder.Build(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building image: %v\n", err)
		os.Exit(1)
	}

	shortName := imgName
	if idx := strings.LastIndex(imgName, "/"); idx >= 0 {
		shortName = imgName[idx+1:]
	}
	fmt.Printf("Successfully tagged %s:%s\n", shortName, imgTag)
}
