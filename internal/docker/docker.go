package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	containertypes "github.com/moby/moby/api/types/container"
	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

type Client struct {
	cli *client.Client
}

type Container struct {
	ID      string
	Names   []string
	Image   string
	ImageID string
	Labels  map[string]string
	State   string
	Project string
	Service string
}

func New(ctx context.Context) (*Client, error) {
	cli, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating Docker client: %w", err)
	}
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		return nil, fmt.Errorf("connecting to Docker daemon: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error { return c.cli.Close() }

func (c *Client) GetRunningContainers(ctx context.Context) ([]Container, error) {
	result, err := c.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	var containers []Container
	for _, s := range result.Items {
		containers = append(containers, Container{
			ID:      s.ID,
			Names:   s.Names,
			Image:   s.Image,
			ImageID: s.ImageID,
			Labels:  s.Labels,
			State:   string(s.State),
			Project: s.Labels["com.docker.compose.project"],
			Service: s.Labels["com.docker.compose.service"],
		})
	}
	return containers, nil
}

func (c *Client) ImageDigest(ctx context.Context, ref string) (string, error) {
	result, err := c.cli.ImageInspect(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("inspecting image %s: %w", ref, err)
	}
	return result.ID, nil
}

func (c *Client) PullImage(ctx context.Context, ref string) error {
	slog.Info("pulling image", "ref", ref)
	resp, err := c.cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling %s: %w", ref, err)
	}
	if _, err := io.Copy(io.Discard, resp); err != nil {
		slog.Warn("discarding pull output", "error", err)
	}
	resp.Close()
	return nil
}

func (c *Client) RemoveImage(ctx context.Context, imageID string) error {
	_, err := c.cli.ImageRemove(ctx, imageID, client.ImageRemoveOptions{PruneChildren: true})
	if err != nil {
		return fmt.Errorf("removing image %s: %w", imageID, err)
	}
	slog.Info("removed old image", "imageID", imageID)
	return nil
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*containertypes.InspectResponse, error) {
	result, err := c.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspecting container %s: %w", id, err)
	}
	return &result.Container, nil
}

func (c *Client) RecreateContainer(ctx context.Context, id, newImage string) (string, error) {
	inspect, err := c.InspectContainer(ctx, id)
	if err != nil {
		return "", fmt.Errorf("inspect: %w", err)
	}

	newCfg := *inspect.Config
	newCfg.Image = newImage
	newCfg.Hostname = ""

	timeout := 10
	if _, err := c.cli.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &timeout}); err != nil {
		return "", fmt.Errorf("stop: %w", err)
	}
	if _, err := c.cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true}); err != nil {
		return "", fmt.Errorf("remove: %w", err)
	}

	name := strings.TrimPrefix(inspect.Name, "/")
	result, err := c.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     &newCfg,
		HostConfig: inspect.HostConfig,
		NetworkingConfig: &networktypes.NetworkingConfig{
			EndpointsConfig: inspect.NetworkSettings.Networks,
		},
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}

	newID := result.ID

	if _, err := c.cli.ContainerStart(ctx, newID, client.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	return newID, nil
}
