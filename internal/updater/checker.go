package updater

import (
	"context"
	"fmt"
	"log"
	"strings"

	dockerclient "github.com/rdoy-lab/dockdater/internal/docker"
)

const enabledLabel = "dockdater.enabled"

type Checker struct {
	docker *dockerclient.Client
}

func NewChecker(dc *dockerclient.Client) *Checker {
	return &Checker{docker: dc}
}

func (c *Checker) CheckAndUpdate(ctx context.Context) error {
	containers, err := c.docker.GetRunningContainers(ctx)
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

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
		log.Printf("Checking %s/%s (image: %s)", ctr.Project, ctr.Service, ref)

		oldID, err := c.docker.ImageDigest(ctx, ref)
		if err != nil {
			log.Printf("Error inspecting %s: %v", ref, err)
			continue
		}

		if err := c.docker.PullImage(ctx, ref); err != nil {
			log.Printf("Error pulling %s: %v", ref, err)
			continue
		}

		newID, err := c.docker.ImageDigest(ctx, ref)
		if err != nil {
			log.Printf("Error inspecting %s after pull: %v", ref, err)
			continue
		}

		if oldID == newID {
			log.Printf("Up-to-date: %s/%s", ctr.Project, ctr.Service)
			continue
		}

		log.Printf("Update available: %s/%s %s -> %s",
			ctr.Project, ctr.Service, shortID(oldID), shortID(newID))
		toUpdate = append(toUpdate, update{ctr: ctr, ref: ref, oldID: oldID, newID: newID})
	}

	if len(toUpdate) == 0 {
		return nil
	}

	log.Printf("Updating %d service(s)...", len(toUpdate))

	for _, u := range toUpdate {
		log.Printf("Recreating %s/%s with %s", u.ctr.Project, u.ctr.Service, u.ref)

		newID, err := c.docker.RecreateContainer(ctx, u.ctr.ID, u.ref)
		if err != nil {
			log.Printf("Error recreating %s/%s: %v", u.ctr.Project, u.ctr.Service, err)
			continue
		}

		log.Printf("Recreated %s/%s -> %s", u.ctr.Project, u.ctr.Service, shortID(newID))

		if u.oldID != u.newID {
			if err := c.docker.RemoveImage(ctx, u.oldID); err != nil {
				log.Printf("Could not remove old image %s: %v", shortID(u.oldID), err)
			}
		}

		log.Printf("Updated %s/%s", u.ctr.Project, u.ctr.Service)
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
