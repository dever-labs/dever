package k8s

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dever-labs/devx/internal/config"
	"gopkg.in/yaml.v3"
)

type ObjectMeta struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

type Deployment struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   ObjectMeta     `yaml:"metadata"`
	Spec       DeploymentSpec `yaml:"spec"`
}

type DeploymentSpec struct {
	Replicas int             `yaml:"replicas"`
	Selector LabelSelector   `yaml:"selector"`
	Template PodTemplateSpec `yaml:"template"`
}

type LabelSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type PodTemplateSpec struct {
	Metadata ObjectMeta `yaml:"metadata"`
	Spec     PodSpec    `yaml:"spec"`
}

type PodSpec struct {
	Containers []Container `yaml:"containers"`
	Volumes    []Volume    `yaml:"volumes,omitempty"`
}

type Container struct {
	Name         string          `yaml:"name"`
	Image        string          `yaml:"image"`
	Command      []string        `yaml:"command,omitempty"`
	WorkingDir   string          `yaml:"workingDir,omitempty"`
	Env          []EnvVar        `yaml:"env,omitempty"`
	Ports        []ContainerPort `yaml:"ports,omitempty"`
	VolumeMounts []VolumeMount   `yaml:"volumeMounts,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type ContainerPort struct {
	ContainerPort int `yaml:"containerPort"`
}

type VolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
}

type Volume struct {
	Name     string    `yaml:"name"`
	EmptyDir *EmptyDir `yaml:"emptyDir,omitempty"`
}

type EmptyDir struct{}

type Service struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   ObjectMeta  `yaml:"metadata"`
	Spec       ServiceSpec `yaml:"spec"`
}

type ServiceSpec struct {
	Selector map[string]string `yaml:"selector"`
	Ports    []ServicePort     `yaml:"ports"`
	Type     string            `yaml:"type,omitempty"`
}

type ServicePort struct {
	Name       string `yaml:"name,omitempty"`
	Port       int    `yaml:"port"`
	TargetPort int    `yaml:"targetPort"`
}

var depImages = map[string]string{
	"postgres": "postgres",
	"redis":    "redis",
}

func Render(manifest *config.Manifest, profileName string, profile *config.Profile, namespace string) (string, error) {
	if manifest == nil || profile == nil {
		return "", fmt.Errorf("manifest and profile are required")
	}

	var docs []any
	project := sanitizeName(manifest.Project.Name)

	for _, name := range sortedKeys(profile.Services) {
		svc := profile.Services[name]
		if svc.Build != nil && svc.Image == "" {
			return "", fmt.Errorf("service '%s' requires image for k8s render", name)
		}
		if len(svc.Mount) > 0 {
			return "", fmt.Errorf("service '%s' uses mount which is not supported in k8s render", name)
		}

		image := svc.Image
		if image == "" {
			return "", fmt.Errorf("service '%s' requires image for k8s render", name)
		}

		labels := map[string]string{"app": project + "-" + sanitizeName(name)}
		container := Container{
			Name:       sanitizeName(name),
			Image:      image,
			Command:    svc.Command,
			WorkingDir: svc.Workdir,
			Env:        envVars(svc.Env),
			Ports:      containerPorts(svc.Ports),
		}

		docs = append(docs, Deployment{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Metadata:   ObjectMeta{Name: labels["app"], Namespace: namespace, Labels: labels},
			Spec: DeploymentSpec{
				Replicas: 1,
				Selector: LabelSelector{MatchLabels: labels},
				Template: PodTemplateSpec{
					Metadata: ObjectMeta{Labels: labels},
					Spec:     PodSpec{Containers: []Container{container}},
				},
			},
		})

		if len(container.Ports) > 0 {
			docs = append(docs, Service{
				APIVersion: "v1",
				Kind:       "Service",
				Metadata:   ObjectMeta{Name: labels["app"], Namespace: namespace, Labels: labels},
				Spec: ServiceSpec{
					Selector: labels,
					Ports:    servicePorts(container.Ports),
				},
			})
		}
	}

	for _, name := range sortedKeys(profile.Deps) {
		dep := profile.Deps[name]
		image := depImages[dep.Kind]
		if image == "" {
			return "", fmt.Errorf("dep '%s' kind '%s' is not supported for k8s render", name, dep.Kind)
		}
		if dep.Version != "" {
			image = image + ":" + dep.Version
		}

		labels := map[string]string{"app": project + "-" + sanitizeName(name)}
		container := Container{
			Name:  sanitizeName(name),
			Image: image,
			Env:   envVars(dep.Env),
			Ports: containerPorts(dep.Ports),
		}

		volumes, mounts, err := depVolumes(name, dep.Volume)
		if err != nil {
			return "", err
		}
		container.VolumeMounts = mounts

		docs = append(docs, Deployment{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Metadata:   ObjectMeta{Name: labels["app"], Namespace: namespace, Labels: labels},
			Spec: DeploymentSpec{
				Replicas: 1,
				Selector: LabelSelector{MatchLabels: labels},
				Template: PodTemplateSpec{
					Metadata: ObjectMeta{Labels: labels},
					Spec:     PodSpec{Containers: []Container{container}, Volumes: volumes},
				},
			},
		})

		if len(container.Ports) > 0 {
			docs = append(docs, Service{
				APIVersion: "v1",
				Kind:       "Service",
				Metadata:   ObjectMeta{Name: labels["app"], Namespace: namespace, Labels: labels},
				Spec: ServiceSpec{
					Selector: labels,
					Ports:    servicePorts(container.Ports),
				},
			})
		}
	}

	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	for i, doc := range docs {
		if i > 0 {
			_, _ = buf.WriteString("---\n")
		}
		if err := enc.Encode(doc); err != nil {
			return "", err
		}
	}

	return buf.String(), nil
}

func envVars(env map[string]string) []EnvVar {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	vars := make([]EnvVar, 0, len(env))
	for _, key := range keys {
		vars = append(vars, EnvVar{Name: key, Value: env[key]})
	}
	return vars
}

func containerPorts(ports []string) []ContainerPort {
	var out []ContainerPort
	seen := map[int]bool{}
	for _, port := range ports {
		cport, err := parseContainerPort(port)
		if err != nil {
			continue
		}
		if seen[cport] {
			continue
		}
		seen[cport] = true
		out = append(out, ContainerPort{ContainerPort: cport})
	}
	return out
}

func servicePorts(ports []ContainerPort) []ServicePort {
	out := make([]ServicePort, 0, len(ports))
	for _, port := range ports {
		name := fmt.Sprintf("p-%d", port.ContainerPort)
		out = append(out, ServicePort{Name: name, Port: port.ContainerPort, TargetPort: port.ContainerPort})
	}
	return out
}

func parseContainerPort(port string) (int, error) {
	parts := strings.Split(port, ":")
	last := parts[len(parts)-1]
	value, err := strconv.Atoi(last)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func depVolumes(depName string, volume string) ([]Volume, []VolumeMount, error) {
	if volume == "" {
		return nil, nil, nil
	}

	parts := strings.SplitN(volume, ":", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("dep '%s' volume must be in name:/path format", depName)
	}

	volName := sanitizeName(depName + "-data")
	mountPath := parts[1]
	return []Volume{{Name: volName, EmptyDir: &EmptyDir{}}}, []VolumeMount{{Name: volName, MountPath: mountPath}}, nil
}

func sanitizeName(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			out.WriteRune(r)
			continue
		}
		out.WriteRune('-')
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		return "devx"
	}
	return result
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
