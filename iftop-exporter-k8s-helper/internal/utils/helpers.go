package utils

import corev1 "k8s.io/api/core/v1"

func IsPodRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == "Running"
}

func IsPodTerminating(pod *corev1.Pod) bool {
	return !pod.ObjectMeta.DeletionTimestamp.IsZero()
}
func PodNodeName(pod *corev1.Pod) string {
	return pod.Spec.NodeName
}

func PodStatus(pod *corev1.Pod) corev1.PodPhase {
	return pod.Status.Phase
}
