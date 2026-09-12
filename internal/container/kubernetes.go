package container

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultNamespace = "foreverbull"
	saNamespaceFile  = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

var dns1123Label = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

type engine struct {
	client    kubernetes.Interface
	namespace string
}

type container struct {
	client    kubernetes.Interface
	namespace string
	name      string
}

// NewEngine builds a Kubernetes engine. In-cluster it uses rest.InClusterConfig
// and the ServiceAccount namespace. Out-of-cluster it loads kubeconfig from
// KUBECONFIG or the default cluster config path (~/.kube/config).
func NewEngine() (Engine, error) {
	cfg, ns, err := restConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %w", err)
	}

	return NewEngineWithClient(client, ns), nil
}

// NewEngineWithClient is used by tests (fake clientset) and by NewEngine.
func NewEngineWithClient(client kubernetes.Interface, namespace string) Engine {
	if namespace == "" {
		namespace = defaultNamespace
	}

	return &engine{client: client, namespace: namespace}
}

func restConfig() (*rest.Config, string, error) {
	if ns, ok := explicitNamespace(); ok {
		cfg, err := loadRESTConfig()
		if err != nil {
			return nil, "", err
		}

		return cfg, ns, nil
	}

	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, inClusterNamespace(), nil
	}

	loader := kubeconfigLoader()

	cfg, err := loader.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("error loading kubernetes config: %w", err)
	}

	ns, _, err := loader.Namespace()
	if err != nil || ns == "" {
		ns = defaultNamespace
	}

	return cfg, ns, nil
}

func loadRESTConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	cfg, err := kubeconfigLoader().ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error loading kubernetes config: %w", err)
	}

	return cfg, nil
}

func kubeconfigLoader() clientcmd.ClientConfig {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
}

func explicitNamespace() (string, bool) {
	for _, key := range []string{"POD_NAMESPACE", "KUBERNETES_NAMESPACE"} {
		if ns := strings.TrimSpace(os.Getenv(key)); ns != "" {
			return ns, true
		}
	}

	return "", false
}

func inClusterNamespace() string {
	data, err := os.ReadFile(saNamespaceFile)
	if err != nil {
		return defaultNamespace
	}

	if ns := strings.TrimSpace(string(data)); ns != "" {
		return ns
	}

	return defaultNamespace
}

func workerLabels(name string) map[string]string {
	return map[string]string{
		LabelPlatform: LabelPlatformValue,
		LabelType:     LabelTypeService,
		LabelAppName:  name,
	}
}

func serviceEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "BROKER_HOSTNAME", Value: environment.GetServerAddress()},
		{Name: "BROKER_PORT", Value: environment.GetHTTPPort()},
		{Name: "STORAGE_ENDPOINT", Value: environment.GetMinioURL()},
		{Name: "STORAGE_ACCESS_KEY", Value: environment.GetMinioAccessKey()},
		{Name: "STORAGE_SECRET_KEY", Value: environment.GetMinioSecretKey()},
		{Name: "DATABASE_URL", Value: environment.GetPostgresURL()},
		{Name: "LOGLEVEL", Value: environment.GetLogLevel()},
	}
}

func (e *engine) wrap(pod *corev1.Pod) Container {
	return &container{
		client:    e.client,
		namespace: e.namespace,
		name:      pod.Name,
	}
}

func (e *engine) Get(ctx context.Context, name string) (Container, error) {
	if name == "" {
		return nil, fmt.Errorf("workload name is empty")
	}

	pods := e.client.CoreV1().Pods(e.namespace)

	pod, err := pods.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return e.wrap(pod), nil
	}

	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("error getting pod %s: %w", name, err)
	}

	list, err := pods.List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{LabelAppName: name}.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("error listing pods for %s: %w", name, err)
	}

	if len(list.Items) == 0 {
		return nil, fmt.Errorf("workload %q not found in namespace %s", name, e.namespace)
	}

	return e.wrap(pickPod(list.Items)), nil
}

func (e *engine) Start(ctx context.Context, image, name string) (Container, error) {
	if image == "" {
		return nil, fmt.Errorf("image is empty")
	}

	if name == "" {
		return nil, fmt.Errorf("name is empty")
	}

	autoMount := false
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: e.namespace,
			Labels:    workerLabels(name),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyAlways,
			AutomountServiceAccountToken: &autoMount,
			Containers: []corev1.Container{{
				Name:            "service",
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env:             serviceEnv(),
				Ports: []corev1.ContainerPort{{
					Name:          "grpc",
					ContainerPort: grpcPort,
					Protocol:      corev1.ProtocolTCP,
				}},
			}},
		},
	}

	if dns1123Label.MatchString(name) {
		pod.Spec.Hostname = name
	}

	created, err := e.client.CoreV1().Pods(e.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creating pod: %w", err)
	}

	return e.wrap(created), nil
}

func (e *engine) StopAll(ctx context.Context, _ bool) error {
	selector := labels.Set{
		LabelPlatform: LabelPlatformValue,
		LabelType:     LabelTypeService,
	}.String()

	pods := e.client.CoreV1().Pods(e.namespace)

	list, err := pods.List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("error listing pods: %w", err)
	}

	for i := range list.Items {
		err := pods.Delete(ctx, list.Items[i].Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error deleting pod %s: %w", list.Items[i].Name, err)
		}
	}

	remaining, err := pods.List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("error listing pods: %w", err)
	}

	if len(remaining.Items) > 0 {
		return fmt.Errorf("expected no service pods, but found %d", len(remaining.Items))
	}

	return nil
}

func (c *container) getPod() (*corev1.Pod, error) {
	pod, err := c.client.CoreV1().Pods(c.namespace).Get(context.Background(), c.name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting pod: %w", err)
	}

	return pod, nil
}

func (c *container) GetStatus() (string, error) {
	pod, err := c.getPod()
	if err != nil {
		return "", err
	}

	if pod.Status.Phase == "" {
		return "pending", nil
	}

	return strings.ToLower(string(pod.Status.Phase)), nil
}

func (c *container) GetHealth() (string, error) {
	pod, err := c.getPod()
	if err != nil {
		return "", err
	}

	return podHealth(pod), nil
}

func (c *container) GetIpAddress() (string, error) {
	pod, err := c.getPod()
	if err != nil {
		return "", err
	}

	if ip := strings.TrimSpace(pod.Status.PodIP); ip != "" {
		return ip, nil
	}

	if app := pod.Labels[LabelAppName]; app != "" {
		return app, nil
	}

	return pod.Name, nil
}

func (c *container) GetConnectionString() (string, error) {
	host, err := c.GetIpAddress()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", host, grpcPort), nil
}

func (c *container) Stop() error {
	err := c.client.CoreV1().Pods(c.namespace).Delete(context.Background(), c.name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error deleting pod: %w", err)
	}

	return nil
}

func pickPod(pods []corev1.Pod) *corev1.Pod {
	for i := range pods {
		if podHealth(&pods[i]) == HealthHealthy {
			return &pods[i]
		}
	}

	return &pods[0]
}

func podHealth(pod *corev1.Pod) string {
	for _, cond := range pod.Status.Conditions {
		if cond.Type != corev1.PodReady {
			continue
		}

		if cond.Status == corev1.ConditionTrue {
			return HealthHealthy
		}

		if pod.Status.Phase == corev1.PodPending {
			return HealthStarting
		}

		return HealthUnhealthy
	}

	switch pod.Status.Phase {
	case corev1.PodRunning:
		return HealthHealthy
	case corev1.PodPending, "":
		return HealthStarting
	case corev1.PodSucceeded, corev1.PodFailed, corev1.PodUnknown:
		return HealthUnhealthy
	default:
		return HealthUnhealthy
	}
}
