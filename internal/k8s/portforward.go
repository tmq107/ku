package k8s

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// ServicePort is the small bit of Service port metadata the UI needs to let the
// user choose a port before starting a forward.
type ServicePort struct {
	Name       string
	Port       int32
	TargetPort string
	Protocol   string
}

// ID returns the value accepted by PortForwardSpec.ServicePort for this port.
func (p ServicePort) ID() string {
	if p.Name != "" {
		return p.Name
	}
	return strconv.Itoa(int(p.Port))
}

// PortForwardSpec describes a service port-forward request. ServicePort is a
// service port name or number. LocalPort 0 means use the selected service port.
type PortForwardSpec struct {
	LocalPort   int32
	ServicePort string
}

// ServicePorts returns the ports exposed by a Service in spec order.
func (c *Client) ServicePorts(ctx context.Context, namespace, name string) ([]ServicePort, error) {
	svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ports := make([]ServicePort, 0, len(svc.Spec.Ports))
	for i := range svc.Spec.Ports {
		ports = append(ports, servicePortInfo(svc.Spec.Ports[i]))
	}
	return ports, nil
}

func servicePortInfo(p corev1.ServicePort) ServicePort {
	return ServicePort{
		Name:       p.Name,
		Port:       p.Port,
		TargetPort: targetPortString(p),
		Protocol:   string(servicePortProtocol(p)),
	}
}

func servicePortProtocol(p corev1.ServicePort) corev1.Protocol {
	if p.Protocol == "" {
		return corev1.ProtocolTCP
	}
	return p.Protocol
}

func targetPortString(p corev1.ServicePort) string {
	if p.TargetPort.Type == intstr.String && p.TargetPort.StrVal != "" {
		return p.TargetPort.StrVal
	}
	if p.TargetPort.IntVal > 0 {
		return strconv.Itoa(int(p.TargetPort.IntVal))
	}
	return strconv.Itoa(int(p.Port))
}

// PortForwardService forwards a local TCP port to a Service by resolving the
// Service selector to a backing pod and forwarding to the Service's targetPort.
// It blocks until stopCh is closed or the stream fails.
func (c *Client) PortForwardService(ctx context.Context, namespace, name string, spec PortForwardSpec, stopCh <-chan struct{}, readyCh chan struct{}, out, errOut io.Writer) error {
	if namespace == "" {
		return fmt.Errorf("service namespace unavailable")
	}
	svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	svcPort, err := resolveServicePort(svc, spec.ServicePort)
	if err != nil {
		return err
	}
	protocol := servicePortProtocol(svcPort)
	if protocol != corev1.ProtocolTCP {
		return fmt.Errorf("port-forward supports TCP services, got %s", svcPort.Protocol)
	}
	pod, err := c.servicePod(ctx, svc)
	if err != nil {
		return err
	}
	remote, err := serviceTargetPortNumber(pod, svcPort)
	if err != nil {
		return err
	}
	local := spec.LocalPort
	if local == 0 {
		local = svcPort.Port
	}
	if out != nil {
		fmt.Fprintf(out, "Forwarding service %s/%s port %s to local %d via pod %s\r\n", namespace, name, servicePortLabel(svcPort), local, pod.Name)
	}
	return c.portForwardPod(namespace, pod.Name, local, remote, stopCh, readyCh, out, errOut)
}

func resolveServicePort(svc *corev1.Service, id string) (corev1.ServicePort, error) {
	ports := svc.Spec.Ports
	if id == "" {
		if len(ports) == 1 {
			return ports[0], nil
		}
		return corev1.ServicePort{}, fmt.Errorf("service %q has %d ports; choose one", svc.Name, len(ports))
	}
	for _, p := range ports {
		if p.Name == id {
			return p, nil
		}
	}
	port, err := strconv.Atoi(id)
	if err == nil {
		for _, p := range ports {
			if int(p.Port) == port {
				return p, nil
			}
		}
	}
	return corev1.ServicePort{}, fmt.Errorf("service %q has no port %q", svc.Name, id)
}

func (c *Client) servicePod(ctx context.Context, svc *corev1.Service) (*corev1.Pod, error) {
	if len(svc.Spec.Selector) == 0 {
		return nil, fmt.Errorf("service %q has no selector", svc.Name)
	}
	selector := labels.SelectorFromSet(labels.Set(svc.Spec.Selector)).String()
	pods, err := c.clientset.CoreV1().Pods(svc.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	items := pods.Items
	sort.Slice(items, func(i, j int) bool {
		ir, jr := podReady(&items[i]), podReady(&items[j])
		if ir != jr {
			return ir
		}
		return items[i].Name < items[j].Name
	})
	for i := range items {
		p := &items[i]
		if p.DeletionTimestamp == nil && p.Status.Phase == corev1.PodRunning {
			return p, nil
		}
	}
	return nil, fmt.Errorf("service %q has no running pods", svc.Name)
}

func serviceTargetPortNumber(pod *corev1.Pod, svcPort corev1.ServicePort) (int32, error) {
	if svcPort.TargetPort.Type != intstr.String {
		if svcPort.TargetPort.IntVal > 0 {
			return svcPort.TargetPort.IntVal, nil
		}
		return svcPort.Port, nil
	}
	name := svcPort.TargetPort.StrVal
	protocol := servicePortProtocol(svcPort)
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == name && (p.Protocol == "" || p.Protocol == protocol) {
				return p.ContainerPort, nil
			}
		}
	}
	return 0, fmt.Errorf("pod %q has no container port named %q", pod.Name, name)
}

func servicePortLabel(p corev1.ServicePort) string {
	if p.Name != "" {
		return fmt.Sprintf("%s (%d)", p.Name, p.Port)
	}
	return strconv.Itoa(int(p.Port))
}

func (c *Client) portForwardPod(namespace, pod string, local, remote int32, stopCh <-chan struct{}, readyCh chan struct{}, out, errOut io.Writer) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(c.restConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	ports := []string{fmt.Sprintf("%d:%d", local, remote)}
	pf, err := portforward.New(dialer, ports, stopCh, readyCh, out, errOut)
	if err != nil {
		return err
	}
	return pf.ForwardPorts()
}
