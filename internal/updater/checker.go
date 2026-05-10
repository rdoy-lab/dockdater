package updater

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	dockerclient "github.com/rdoy-lab/dockdater/internal/docker"
	"github.com/rdoy-lab/dockdater/internal/state"
)

const enabledLabel = "dockdater.enabled"

type Checker struct {
	docker *dockerclient.Client
	store  *state.Store
}

func NewChecker(dc *dockerclient.Client, store *state.Store) *Checker {
	return &Checker{docker: dc, store: store}
}

func (c *Checker) CheckAndUpdate(ctx context.Context) error {
	containers, err := c.docker.GetRunningContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	now := time.Now()

	rows := make([]state.ContainerRow, 0, len(containers))
	for _, ctr := range containers {
		rows = append(rows, state.ContainerRow{
			ID:      ctr.ID,
			Names:   ctr.Names,
			Image:   ctr.Image,
			State:   ctr.State,
			Project: ctr.Project,
			Service: ctr.Service,
			Enabled: ctr.Labels[enabledLabel] == "true",
		})
	}
	c.store.UpdateContainers(rows, now)

	type update struct {
		ctr   dockerclient.Container
		ref   string
		oldID string
		newID string
	}

	var toUpdate []update

	for _, ctr := range containers {
		if ctr.Labels[enabledLabel] != "true" {
			continue
		}

		ref := normalizeRef(ctr.Image)
		slog.Debug("checking image", "project", ctr.Project, "service", ctr.Service, "ref", ref)

		oldID, err := c.docker.ImageDigest(ctx, ref)
		if err != nil {
			slog.Error("inspecting image", "ref", ref, "error", err)
			c.store.MarkContainerError(ctr.ID)
			continue
		}

		if err := c.docker.PullImage(ctx, ref); err != nil {
			slog.Error("pulling image", "ref", ref, "error", err)
			c.store.MarkContainerError(ctr.ID)
			continue
		}

		newID, err := c.docker.ImageDigest(ctx, ref)
		if err != nil {
			slog.Error("inspecting image after pull", "ref", ref, "error", err)
			c.store.MarkContainerError(ctr.ID)
			continue
		}

		if oldID == newID {
			slog.Debug("image already up-to-date", "project", ctr.Project, "service", ctr.Service, "ref", ref)
			c.store.MarkContainerChecked(ctr.ID, true)
			continue
		}

		slog.Info("update available", "project", ctr.Project, "service", ctr.Service, "old", shortID(oldID), "new", shortID(newID))
		toUpdate = append(toUpdate, update{ctr: ctr, ref: ref, oldID: oldID, newID: newID})
	}

	if len(toUpdate) == 0 {
		return nil
	}

	slog.Info("updating services", "count", len(toUpdate))

	for _, u := range toUpdate {
		slog.Info("recreating container", "project", u.ctr.Project, "service", u.ctr.Service, "ref", u.ref)

		newID, err := c.docker.RecreateContainer(ctx, u.ctr.ID, u.ref)
		if err != nil {
			slog.Error("recreating container", "project", u.ctr.Project, "service", u.ctr.Service, "error", err)
			c.store.MarkContainerError(u.ctr.ID)
			continue
		}

		slog.Info("recreated container", "project", u.ctr.Project, "service", u.ctr.Service, "newID", shortID(newID))
		c.store.MarkContainerChecked(newID, true)
		c.store.AddDeployment(state.DeploymentRow{
			Time:    time.Now().Format(time.RFC3339),
			Project: u.ctr.Project,
			Service: u.ctr.Service,
			Image:   u.ref,
			OldRef:  u.oldID,
			NewRef:  u.newID,
		})

		if u.oldID != u.newID {
			if err := c.docker.RemoveImage(ctx, u.oldID); err != nil {
				slog.Warn("could not remove old image", "oldID", shortID(u.oldID), "error", err)
			}
		}

		slog.Info("updated service", "project", u.ctr.Project, "service", u.ctr.Service)
	}

	return nil
}

func normalizeRef(ref string) string {
	if idx := strings.Index(ref, "@sha256:"); idx != -1 {
		ref = ref[:idx]
	}
	parts := strings.Split(ref, "/")
	last := parts[len(parts)-1]
	if !strings.Contains(last, ":") {
		return ref + ":latest"
	}
	return ref
}

func shortID(id string) string {
	if len(id) > 19 && strings.HasPrefix(id, "sha256:") {
		return id[:19] + "..."
	}
	if len(id) > 16 {
		return id[:16] + "..."
	}
	return id
}
