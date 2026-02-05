//go:build darwin

package llm_resolver

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	// WireGuard configuration
	hostPeerIP    = "10.33.33.1"
	vmPeerIP      = "10.33.33.2"
	wireguardPort = 3333

	// Setup container image
	setupImage = "chipmk/docker-mac-net-connect-setup:latest"
)

// NetworkTunnel manages the WireGuard tunnel to Docker VM on macOS
type NetworkTunnel struct {
	logger         *zap.Logger
	device         *device.Device
	uapi           net.Listener
	tunDevice      tun.Device
	interfaceName  string
	dockerNetworks map[string]network.Inspect
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	running        bool
}

// NewNetworkTunnel creates a new network tunnel manager
func NewNetworkTunnel(logger *zap.Logger) *NetworkTunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &NetworkTunnel{
		logger:         logger,
		dockerNetworks: make(map[string]network.Inspect),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start initializes the WireGuard tunnel to Docker VM
func (nt *NetworkTunnel) Start() error {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	if nt.running {
		return nil
	}

	// Check if we're running as root
	if os.Geteuid() != 0 {
		nt.logger.Warn("network tunnel requires root privileges, skipping")
		return nil
	}

	// Check if Docker is available
	dockerCli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		nt.logger.Warn("docker not available, skipping network tunnel", zap.Error(err))
		return nil
	}
	defer dockerCli.Close()

	// Ping Docker to make sure it's running
	_, err = dockerCli.Ping(nt.ctx)
	if err != nil {
		nt.logger.Warn("docker not running, skipping network tunnel", zap.Error(err))
		return nil
	}

	nt.logger.Info("starting network tunnel to Docker VM")

	// Create TUN device
	tunDev, err := tun.CreateTUN("utun", device.DefaultMTU)
	if err != nil {
		return fmt.Errorf("failed to create TUN device: %w", err)
	}
	nt.tunDevice = tunDev

	interfaceName, err := tunDev.Name()
	if err != nil {
		tunDev.Close()
		return fmt.Errorf("failed to get TUN device name: %w", err)
	}
	nt.interfaceName = interfaceName

	// Create WireGuard device
	wgLogger := device.NewLogger(device.LogLevelError, fmt.Sprintf("(%s) ", interfaceName))
	nt.device = device.NewDevice(tunDev, conn.NewDefaultBind(), wgLogger)

	// Setup UAPI
	fileUAPI, err := ipc.UAPIOpen(interfaceName)
	if err != nil {
		nt.device.Close()
		return fmt.Errorf("failed to open UAPI: %w", err)
	}

	nt.uapi, err = ipc.UAPIListen(interfaceName, fileUAPI)
	if err != nil {
		nt.device.Close()
		return fmt.Errorf("failed to listen on UAPI: %w", err)
	}

	// Handle UAPI connections
	go func() {
		for {
			conn, err := nt.uapi.Accept()
			if err != nil {
				return
			}
			go nt.device.IpcHandle(conn)
		}
	}()

	// Configure WireGuard
	if err := nt.configureWireGuard(); err != nil {
		nt.Stop()
		return fmt.Errorf("failed to configure WireGuard: %w", err)
	}

	// Set interface address
	if err := nt.setInterfaceAddress(); err != nil {
		nt.Stop()
		return fmt.Errorf("failed to set interface address: %w", err)
	}

	nt.running = true
	nt.logger.Info("network tunnel interface created", zap.String("interface", interfaceName))

	// Start background goroutine to manage Docker networks
	go nt.manageDockerNetworks()

	return nil
}

// configureWireGuard sets up the WireGuard keys and peer configuration
func (nt *NetworkTunnel) configureWireGuard() error {
	wgClient, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to create wgctrl client: %w", err)
	}
	defer wgClient.Close()

	// Generate keys
	hostPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate host private key: %w", err)
	}

	vmPrivateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return fmt.Errorf("failed to generate VM private key: %w", err)
	}

	// Store keys for VM setup
	nt.setupVMKeys(hostPrivateKey, vmPrivateKey)

	// Configure allowed IPs
	_, wildcardNet, _ := net.ParseCIDR("0.0.0.0/0")
	_, vmNet, _ := net.ParseCIDR(vmPeerIP + "/32")

	peer := wgtypes.PeerConfig{
		PublicKey: vmPrivateKey.PublicKey(),
		AllowedIPs: []net.IPNet{
			*wildcardNet,
			*vmNet,
		},
	}

	port := wireguardPort
	err = wgClient.ConfigureDevice(nt.interfaceName, wgtypes.Config{
		ListenPort: &port,
		PrivateKey: &hostPrivateKey,
		Peers:      []wgtypes.PeerConfig{peer},
	})
	if err != nil {
		return fmt.Errorf("failed to configure WireGuard device: %w", err)
	}

	return nil
}

// setupVMKeys stores the keys temporarily for VM setup
var vmSetupKeys struct {
	hostPrivateKey wgtypes.Key
	vmPrivateKey   wgtypes.Key
}

func (nt *NetworkTunnel) setupVMKeys(hostKey, vmKey wgtypes.Key) {
	vmSetupKeys.hostPrivateKey = hostKey
	vmSetupKeys.vmPrivateKey = vmKey
}

// setInterfaceAddress configures the TUN interface IP address
func (nt *NetworkTunnel) setInterfaceAddress() error {
	cmd := exec.Command("ifconfig", nt.interfaceName, "inet", hostPeerIP+"/32", vmPeerIP)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ifconfig failed: %s: %w", string(output), err)
	}
	return nil
}

// manageDockerNetworks watches for Docker network events and manages routes
func (nt *NetworkTunnel) manageDockerNetworks() {
	for {
		select {
		case <-nt.ctx.Done():
			return
		default:
		}

		if err := nt.setupAndWatch(); err != nil {
			nt.logger.Error("docker network watcher error", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
	}
}

func (nt *NetworkTunnel) setupAndWatch() error {
	dockerCli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerCli.Close()

	// Setup WireGuard on VM side
	nt.logger.Info("setting up WireGuard on Docker Desktop VM")
	if err := nt.setupVM(dockerCli); err != nil {
		return fmt.Errorf("failed to setup VM: %w", err)
	}

	// Get existing networks and add routes
	networks, err := dockerCli.NetworkList(nt.ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Docker networks: %w", err)
	}

	for _, network := range networks {
		nt.processNetworkCreate(network)
	}

	nt.logger.Info("watching Docker network events")

	// Watch for network events
	msgs, errs := dockerCli.Events(nt.ctx, events.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("type", "network"),
			filters.Arg("event", "create"),
			filters.Arg("event", "destroy"),
		),
	})

	for {
		select {
		case <-nt.ctx.Done():
			return nil
		case err := <-errs:
			return err
		case msg := <-msgs:
			if msg.Type == "network" && msg.Action == "create" {
				network, err := dockerCli.NetworkInspect(nt.ctx, msg.Actor.ID, network.InspectOptions{})
				if err != nil {
					nt.logger.Error("failed to inspect network", zap.Error(err))
					continue
				}
				nt.processNetworkCreate(network)
			}

			if msg.Type == "network" && msg.Action == "destroy" {
				nt.mu.RLock()
				network, exists := nt.dockerNetworks[msg.Actor.ID]
				nt.mu.RUnlock()
				if exists {
					nt.processNetworkDestroy(network)
				}
			}
		}
	}
}

// setupVM runs the setup container to configure WireGuard on the Docker VM
func (nt *NetworkTunnel) setupVM(dockerCli *client.Client) error {
	ctx := nt.ctx

	// Pull image if needed
	_, _, err := dockerCli.ImageInspectWithRaw(ctx, setupImage)
	if err != nil {
		nt.logger.Info("pulling setup image", zap.String("image", setupImage))
		pullStream, err := dockerCli.ImagePull(ctx, setupImage, image.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull setup image: %w", err)
		}
		io.Copy(io.Discard, pullStream)
		pullStream.Close()
	}

	// Create and start setup container
	resp, err := dockerCli.ContainerCreate(ctx, &container.Config{
		Image: setupImage,
		Env: []string{
			"SERVER_PORT=" + strconv.Itoa(wireguardPort),
			"HOST_PEER_IP=" + hostPeerIP,
			"VM_PEER_IP=" + vmPeerIP,
			"HOST_PUBLIC_KEY=" + vmSetupKeys.hostPrivateKey.PublicKey().String(),
			"VM_PRIVATE_KEY=" + vmSetupKeys.vmPrivateKey.String(),
		},
	}, &container.HostConfig{
		AutoRemove:  true,
		NetworkMode: "host",
		Privileged:  true,
		CapAdd:      []string{"NET_ADMIN"},
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create setup container: %w", err)
	}

	if err := dockerCli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start setup container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := dockerCli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for setup container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("setup container exited with code %d", status.StatusCode)
		}
	}

	nt.logger.Info("WireGuard configured on Docker VM")
	return nil
}

// processNetworkCreate adds routes for a new Docker network
func (nt *NetworkTunnel) processNetworkCreate(network network.Inspect) {
	nt.mu.Lock()
	nt.dockerNetworks[network.ID] = network
	nt.mu.Unlock()

	for _, config := range network.IPAM.Config {
		if network.Scope == "local" && config.Subnet != "" {
			nt.logger.Info("adding route for Docker network",
				zap.String("subnet", config.Subnet),
				zap.String("network", network.Name),
				zap.String("interface", nt.interfaceName))

			cmd := exec.Command("route", "-q", "-n", "add", "-inet", config.Subnet, "-interface", nt.interfaceName)
			if output, err := cmd.CombinedOutput(); err != nil {
				// Ignore "already exists" errors
				nt.logger.Debug("route add output", zap.String("output", string(output)))
			}
		}
	}
}

// processNetworkDestroy removes routes for a destroyed Docker network
func (nt *NetworkTunnel) processNetworkDestroy(network network.Inspect) {
	for _, config := range network.IPAM.Config {
		if network.Scope == "local" && config.Subnet != "" {
			nt.logger.Info("removing route for Docker network",
				zap.String("subnet", config.Subnet),
				zap.String("network", network.Name))

			cmd := exec.Command("route", "-q", "-n", "delete", "-inet", config.Subnet)
			cmd.Run()
		}
	}

	nt.mu.Lock()
	delete(nt.dockerNetworks, network.ID)
	nt.mu.Unlock()
}

// Stop shuts down the network tunnel
func (nt *NetworkTunnel) Stop() {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	if !nt.running {
		return
	}

	nt.logger.Info("stopping network tunnel")

	// Cancel context to stop goroutines
	nt.cancel()

	// Remove all routes
	for _, network := range nt.dockerNetworks {
		for _, config := range network.IPAM.Config {
			if config.Subnet != "" {
				exec.Command("route", "-q", "-n", "delete", "-inet", config.Subnet).Run()
			}
		}
	}

	// Close UAPI
	if nt.uapi != nil {
		nt.uapi.Close()
	}

	// Close WireGuard device
	if nt.device != nil {
		nt.device.Close()
	}

	// Close TUN device
	if nt.tunDevice != nil {
		nt.tunDevice.Close()
	}

	nt.running = false
	nt.logger.Info("network tunnel stopped")
}

// IsRunning returns whether the tunnel is active
func (nt *NetworkTunnel) IsRunning() bool {
	nt.mu.RLock()
	defer nt.mu.RUnlock()
	return nt.running
}
