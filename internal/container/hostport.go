//go:build linux

package container

// hostport.go — проверка конфликтов host-портов между контейнерами.
//
// Почему это важно: опубликованный порт пробрасывается правилом iptables
// DNAT. Правило для одного и того же host-порта может существовать только
// одно: при старте второго контейнера старое правило удаляется и создаётся
// новое (см. network.AddPortForwarding → removeExistingDNAT). То есть второй
// контейнер МОЛЧА перехватывает трафик первого.
//
// CLI это проверял (cmd/hostport.go), а HTTP API — нет: именно через API
// работают wings и панель, то есть путь, которым пользуются все.

// PortOwner — контейнер, уже занявший host-порт.
type PortOwner struct {
	Container *Container
	Mapping   *PortMap
}

// FindPortOwner возвращает контейнер (кроме selfID/selfName), который уже
// занимает hostPort по протоколу proto.
func FindPortOwner(hostPort int, protocol, selfID, selfName string) *PortOwner {
	others, err := List(true)
	if err != nil {
		return nil
	}
	for _, o := range others {
		if selfID != "" && o.ID == selfID {
			continue
		}
		if selfName != "" && o.Name == selfName {
			continue
		}
		if pm := o.FindPort(hostPort, protocol); pm != nil {
			return &PortOwner{Container: o, Mapping: pm}
		}
	}
	return nil
}

// PortConflict описывает конфликт host-порта.
type PortConflict struct {
	HostPort      int
	Protocol      string
	OwnerName     string
	OwnerID       string
	ContainerPort int
}

// CheckPortConflicts возвращает первый конфликт host-порта (nil, если всё
// свободно). selfID/selfName — контейнер, который создаётся (его же
// собственные маппинги не считаются конфликтом).
func CheckPortConflicts(ports []PortMap, selfID, selfName string) *PortConflict {
	for _, p := range ports {
		if p.HostPort == 0 {
			continue
		}
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		if owner := FindPortOwner(p.HostPort, proto, selfID, selfName); owner != nil {
			return &PortConflict{
				HostPort:      p.HostPort,
				Protocol:      proto,
				OwnerName:     owner.Container.Name,
				OwnerID:       owner.Container.ID,
				ContainerPort: owner.Mapping.ContainerPort,
			}
		}
	}
	return nil
}
