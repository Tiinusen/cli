//go:build !windows && !linux

package cli

// Daemonize makes the application work as a linux/windows service
func (b *Builder) Daemonize(serviceName string, serviceDescription string) *Builder {
	if b.daemoize {
		return b
	}
	b.daemoize = true
	return b
}
