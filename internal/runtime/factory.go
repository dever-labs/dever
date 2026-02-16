package runtime

import (
    "context"

    "github.com/dever-labs/devx/internal/runtime/docker"
    "github.com/dever-labs/devx/internal/runtime/podman"
)

func NewDocker() Runtime {
    return docker.New()
}

func NewPodman() Runtime {
    return podman.New()
}

func DetectAll(ctx context.Context) []RuntimeInfo {
    var infos []RuntimeInfo

    dockerRT := docker.New()
    infos = append(infos, detectRuntime(ctx, dockerRT))

    podmanRT := podman.New()
    infos = append(infos, detectRuntime(ctx, podmanRT))

    return infos
}

func detectRuntime(ctx context.Context, rt Runtime) RuntimeInfo {
    ok, err := rt.Detect(ctx)
    info := RuntimeInfo{
        Name:      rt.Name(),
        Available: ok,
    }
    if err != nil {
        info.Details = err.Error()
    }
    return info
}
