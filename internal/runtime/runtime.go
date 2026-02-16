package runtime

import "context"

type UpOptions struct {
    Build bool
    Pull  bool
}

type LogsOptions struct {
    Service string
    Follow  bool
    Since   string
    JSON    bool
}

type ServiceStatus struct {
    Name   string
    State  string
    Health string
    Ports  string
}

type Runtime interface {
    Name() string
    Detect(ctx context.Context) (bool, error)
    Up(ctx context.Context, composePath string, projectName string, opts UpOptions) error
    Down(ctx context.Context, composePath string, projectName string, removeVolumes bool) error
    Logs(ctx context.Context, composePath string, projectName string, opts LogsOptions) (ReadCloser, error)
    Exec(ctx context.Context, composePath string, projectName string, service string, cmd []string) (int, error)
    Status(ctx context.Context, composePath string, projectName string) ([]ServiceStatus, error)
}

type DigestResolver interface {
    ResolveImageDigest(ctx context.Context, image string) (string, error)
}

type ReadCloser interface {
    Read(p []byte) (int, error)
    Close() error
}

type RuntimeInfo struct {
    Name      string
    Available bool
    Version   string
    Compose   bool
    Details   string
}

func SelectRuntime(ctx context.Context) (Runtime, error) {
    docker := NewDocker()
    if ok, _ := docker.Detect(ctx); ok {
        return docker, nil
    }

    podman := NewPodman()
    if ok, _ := podman.Detect(ctx); ok {
        return podman, nil
    }

    return nil, ErrNoRuntime
}

var ErrNoRuntime = &RuntimeError{Message: "no container runtime detected"}

type RuntimeError struct {
    Message string
}

func (e *RuntimeError) Error() string {
    return e.Message
}
