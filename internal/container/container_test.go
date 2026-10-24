package container

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "foreverbull"

func testEngine(t *testing.T, objects ...runtime.Object) Engine {
	t.Helper()
	environment.SetupForTest(t)

	return NewEngineWithClient(fake.NewSimpleClientset(objects...), testNamespace)
}

func envValue(t *testing.T, env []corev1.EnvVar, name string) string {
	t.Helper()

	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}

	t.Fatalf("missing env %s", name)

	return ""
}

func TestStartCreatesLabeledPodWithBrokerEnv(t *testing.T) {
	environment.SetupForTest(t)

	eng := testEngine(t)
	got, err := eng.Start(context.Background(), "lhjnilsson/strategy:local", "worker-1")
	require.NoError(t, err)
	require.NotNil(t, got)

	pod, err := eng.(*engine).client.CoreV1().Pods(testNamespace).Get(
		context.Background(), "worker-1", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Equal(t, LabelPlatformValue, pod.Labels[LabelPlatform])
	require.Equal(t, LabelTypeService, pod.Labels[LabelType])
	require.Equal(t, "worker-1", pod.Labels[LabelAppName])
	require.Equal(t, "lhjnilsson/strategy:local", pod.Spec.Containers[0].Image)
	require.Equal(t, "worker-1", pod.Spec.Hostname)

	env := pod.Spec.Containers[0].Env
	require.Equal(t, environment.GetServerAddress(), envValue(t, env, "BROKER_HOSTNAME"))
	require.Equal(t, environment.GetGRPCPort(), envValue(t, env, "BROKER_PORT"))
	require.Equal(t, environment.GetLogLevel(), envValue(t, env, "LOGLEVEL"))
	require.False(t, envSet(env, OHLCProviderEnv))
	requireServicePodOmitsBrokerCredentials(t, env)
}

func requireServicePodOmitsBrokerCredentials(t *testing.T, env []corev1.EnvVar) {
	t.Helper()

	for _, name := range []string{"DATABASE_URL", "STORAGE_ENDPOINT", "STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY"} {
		require.Falsef(t, envSet(env, name), "service pod env must not set %s", name)
	}

	brokerURL := environment.GetPostgresURL()
	minioKey := environment.GetMinioAccessKey()
	minioSecret := environment.GetMinioSecretKey()
	for _, item := range env {
		require.NotEqual(t, brokerURL, item.Value)
		require.NotContains(t, item.Value, minioSecret)
		if item.Name != "BROKER_HOSTNAME" && item.Name != "BROKER_PORT" && item.Name != "LOGLEVEL" && item.Name != OHLCProviderEnv {
			require.NotEqual(t, minioKey, item.Value)
		}
	}
}

func TestStartSetsOHLCProviderFromContext(t *testing.T) {
	environment.SetupForTest(t)

	eng := testEngine(t)
	ctx := WithOHLCProvider(context.Background(), "stooq")
	_, err := eng.Start(ctx, "lhjnilsson/strategy:local", "worker-stooq")
	require.NoError(t, err)

	pod, err := eng.(*engine).client.CoreV1().Pods(testNamespace).Get(
		context.Background(), "worker-stooq", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Equal(t, "stooq", envValue(t, pod.Spec.Containers[0].Env, OHLCProviderEnv))
	requireServicePodOmitsBrokerCredentials(t, pod.Spec.Containers[0].Env)
}

func envSet(env []corev1.EnvVar, name string) bool {
	for _, item := range env {
		if item.Name == name {
			return true
		}
	}
	return false
}

func TestGetByNameAndLabel(t *testing.T) {
	zipline := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zipline-7d9c8b-abcde",
			Namespace: testNamespace,
			Labels: map[string]string{
				LabelAppName:  "zipline",
				LabelPlatform: LabelPlatformValue,
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.1.2.3",
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	eng := testEngine(t, zipline)

	byLabel, err := eng.Get(context.Background(), "zipline")
	require.NoError(t, err)

	conn, err := byLabel.GetConnectionString()
	require.NoError(t, err)
	require.Equal(t, "10.1.2.3:50055", conn)

	ip, err := byLabel.GetIpAddress()
	require.NoError(t, err)
	require.Equal(t, "10.1.2.3", ip)

	health, err := byLabel.GetHealth()
	require.NoError(t, err)
	require.Equal(t, HealthHealthy, health)

	status, err := byLabel.GetStatus()
	require.NoError(t, err)
	require.Equal(t, "running", status)

	byName, err := eng.Get(context.Background(), "zipline-7d9c8b-abcde")
	require.NoError(t, err)
	conn, err = byName.GetConnectionString()
	require.NoError(t, err)
	require.Equal(t, "10.1.2.3:50055", conn)
}

func TestGetZiplineFallsBackToServiceDNS(t *testing.T) {
	zipline := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zipline-aaaa",
			Namespace: testNamespace,
			Labels: map[string]string{
				LabelAppName: "zipline",
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	eng := testEngine(t, zipline)
	got, err := eng.Get(context.Background(), "zipline")
	require.NoError(t, err)

	conn, err := got.GetConnectionString()
	require.NoError(t, err)
	require.Equal(t, "zipline:50055", conn)
}

func TestGetMissingWorkload(t *testing.T) {
	eng := testEngine(t)
	_, err := eng.Get(context.Background(), "missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestGetPrefersReadyReplica(t *testing.T) {
	pending := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zipline-pending",
			Namespace: testNamespace,
			Labels:    map[string]string{LabelAppName: "zipline"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending, PodIP: "10.0.0.1"},
	}
	ready := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zipline-ready",
			Namespace: testNamespace,
			Labels:    map[string]string{LabelAppName: "zipline"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	eng := testEngine(t, pending, ready)
	got, err := eng.Get(context.Background(), "zipline")
	require.NoError(t, err)

	ip, err := got.GetIpAddress()
	require.NoError(t, err)
	require.Equal(t, "10.0.0.2", ip)
}

func TestHealthFromPodConditions(t *testing.T) {
	cases := []struct {
		name   string
		status corev1.PodStatus
		want   string
	}{
		{
			name: "ready",
			status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}},
			},
			want: HealthHealthy,
		},
		{
			name:   "pending",
			status: corev1.PodStatus{Phase: corev1.PodPending},
			want:   HealthStarting,
		},
		{
			name: "unready-running",
			status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				}},
			},
			want: HealthUnhealthy,
		},
		{
			name:   "failed",
			status: corev1.PodStatus{Phase: corev1.PodFailed},
			want:   HealthUnhealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: testNamespace},
				Status:     tc.status,
			}
			eng := testEngine(t, pod)
			got, err := eng.Get(context.Background(), "p")
			require.NoError(t, err)
			health, err := got.GetHealth()
			require.NoError(t, err)
			require.Equal(t, tc.want, health)
		})
	}
}

func TestStopDeletesPod(t *testing.T) {
	eng := testEngine(t)
	got, err := eng.Start(context.Background(), "img:tag", "svc-a")
	require.NoError(t, err)
	require.NoError(t, got.Stop())

	_, err = eng.(*engine).client.CoreV1().Pods(testNamespace).Get(
		context.Background(), "svc-a", metav1.GetOptions{},
	)
	require.True(t, apierrors.IsNotFound(err))
}

func TestStopAllDeletesServicePodsOnly(t *testing.T) {
	zipline := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zipline-keep",
			Namespace: testNamespace,
			Labels: map[string]string{
				LabelAppName:  "zipline",
				LabelPlatform: LabelPlatformValue,
			},
		},
	}
	otherNS := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-other",
			Namespace: "other",
			Labels:    workerLabels("worker-other"),
		},
	}

	eng := testEngine(t, zipline, otherNS)
	_, err := eng.Start(context.Background(), "img:tag", "worker-a")
	require.NoError(t, err)
	_, err = eng.Start(context.Background(), "img:tag", "worker-b")
	require.NoError(t, err)

	require.NoError(t, eng.StopAll(context.Background(), true))

	client := eng.(*engine).client
	_, err = client.CoreV1().Pods(testNamespace).Get(context.Background(), "worker-a", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))
	_, err = client.CoreV1().Pods(testNamespace).Get(context.Background(), "worker-b", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err))

	kept, err := client.CoreV1().Pods(testNamespace).Get(context.Background(), "zipline-keep", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "zipline-keep", kept.Name)

	other, err := client.CoreV1().Pods("other").Get(context.Background(), "worker-other", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "worker-other", other.Name)
}

func TestNewEngineWithClientDefaultNamespace(t *testing.T) {
	eng := NewEngineWithClient(fake.NewSimpleClientset(), "")
	require.Equal(t, defaultNamespace, eng.(*engine).namespace)
}

func TestNewEngineFromKubeconfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:1
    insecure-skip-tls-verify: true
  name: fake
contexts:
- context:
    cluster: fake
    namespace: from-kubeconfig
    user: fake
  name: fake
current-context: fake
users:
- name: fake
  user:
    token: fake
`), 0o600))

	t.Setenv("KUBECONFIG", path)
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("POD_NAMESPACE", "")
	t.Setenv("KUBERNETES_NAMESPACE", "")

	eng, err := NewEngine()
	require.NoError(t, err)
	require.Equal(t, "from-kubeconfig", eng.(*engine).namespace)
}

func TestNewEngineNamespaceEnvOverridesKubeconfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(`
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:1
    insecure-skip-tls-verify: true
  name: fake
contexts:
- context:
    cluster: fake
    namespace: from-kubeconfig
    user: fake
  name: fake
current-context: fake
users:
- name: fake
  user:
    token: fake
`), 0o600))

	t.Setenv("KUBECONFIG", path)
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("POD_NAMESPACE", "override-ns")

	eng, err := NewEngine()
	require.NoError(t, err)
	require.Equal(t, "override-ns", eng.(*engine).namespace)
}

func TestNewEngineMissingKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	t.Setenv("POD_NAMESPACE", "")
	t.Setenv("KUBERNETES_NAMESPACE", "")

	_, err := NewEngine()
	require.Error(t, err)
}

func TestGetEmptyName(t *testing.T) {
	_, err := testEngine(t).Get(context.Background(), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestStartRejectsEmptyImageAndName(t *testing.T) {
	eng := testEngine(t)

	_, err := eng.Start(context.Background(), "", "worker-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image is empty")

	_, err = eng.Start(context.Background(), "img:tag", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is empty")
}

func TestStartDoesNotMountDockerSock(t *testing.T) {
	eng := testEngine(t)
	_, err := eng.Start(context.Background(), "lhjnilsson/strategy:local", "worker-1")
	require.NoError(t, err)

	pod, err := eng.(*engine).client.CoreV1().Pods(testNamespace).Get(
		context.Background(), "worker-1", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Empty(t, pod.Spec.Volumes)
	require.Empty(t, pod.Spec.Containers[0].VolumeMounts)
	require.NotNil(t, pod.Spec.AutomountServiceAccountToken)
	require.False(t, *pod.Spec.AutomountServiceAccountToken)

	for _, vol := range pod.Spec.Volumes {
		if vol.HostPath != nil {
			require.NotEqual(t, "/var/run/docker.sock", vol.HostPath.Path)
		}
	}
}

func TestStartSkipsInvalidHostname(t *testing.T) {
	eng := testEngine(t)
	_, err := eng.Start(context.Background(), "img:tag", "Not_Valid")
	require.NoError(t, err)

	pod, err := eng.(*engine).client.CoreV1().Pods(testNamespace).Get(
		context.Background(), "Not_Valid", metav1.GetOptions{},
	)
	require.NoError(t, err)
	require.Empty(t, pod.Spec.Hostname)
}

func TestGetConnectionStringErrorsWhenPodMissing(t *testing.T) {
	eng := testEngine(t)
	got, err := eng.Start(context.Background(), "img:tag", "worker-gone")
	require.NoError(t, err)
	require.NoError(t, got.Stop())

	_, err = got.GetConnectionString()
	require.Error(t, err)
	require.Contains(t, err.Error(), "error getting pod")

	_, err = got.GetStatus()
	require.Error(t, err)
}

func TestStopAllWithNoServicePods(t *testing.T) {
	eng := testEngine(t)
	require.NoError(t, eng.StopAll(context.Background(), true))
}

func TestGetMissingPodByExactName(t *testing.T) {
	eng := testEngine(t)
	_, err := eng.Get(context.Background(), "no-such-pod")
	require.Error(t, err)
	require.Contains(t, err.Error(), `workload "no-such-pod" not found`)
}

func ziplinePod(name, ip string, ready, terminating bool) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{LabelAppName: "zipline"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending, PodIP: ip},
	}
	if ready {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		}}
	}
	if terminating {
		deleted := metav1.Now()
		pod.DeletionTimestamp = &deleted
	}
	return pod
}

func TestPickPodSkipsTerminatingPods(t *testing.T) {
	cases := []struct {
		name string
		pods []corev1.Pod
		want string
	}{
		{
			name: "terminating ready pod listed first loses to the new ready pod",
			pods: []corev1.Pod{
				ziplinePod("zipline-old", "10.42.0.39", true, true),
				ziplinePod("zipline-new", "10.42.0.41", true, false),
			},
			want: "zipline-new",
		},
		{
			name: "terminating ready pod loses to a new pod that is still starting",
			pods: []corev1.Pod{
				ziplinePod("zipline-old", "10.42.0.39", true, true),
				ziplinePod("zipline-new", "", false, false),
			},
			want: "zipline-new",
		},
		{
			name: "ready pod wins over a starting pod",
			pods: []corev1.Pod{
				ziplinePod("zipline-starting", "", false, false),
				ziplinePod("zipline-ready", "10.42.0.41", true, false),
			},
			want: "zipline-ready",
		},
		{
			name: "only a terminating pod is left",
			pods: []corev1.Pod{
				ziplinePod("zipline-old", "10.42.0.39", true, true),
			},
			want: "zipline-old",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, pickPod(tc.pods).Name)
		})
	}
}

func TestGetByLabelSkipsTerminatingZiplinePod(t *testing.T) {
	old := ziplinePod("zipline-a-old", "10.42.0.39", true, true)
	current := ziplinePod("zipline-b-new", "10.42.0.41", true, false)

	eng := testEngine(t, &old, &current)
	got, err := eng.Get(context.Background(), "zipline")
	require.NoError(t, err)

	ip, err := got.GetIpAddress()
	require.NoError(t, err)
	require.Equal(t, "10.42.0.41", ip)
}
