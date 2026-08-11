//go:build linux

package container

import "fmt"

func (c *Container) Rename(newName string) error {
	if c.Name == newName {
		return nil
	}

	existing := FindByName(newName)
	if existing != nil {
		return fmt.Errorf("a container with name %q already exists", newName)
	}

	c.Name = newName
	return c.Save()
}
