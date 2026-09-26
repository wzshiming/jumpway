<script lang="ts">
	// A placeholder bar sized by the caller; the wrapper carries aria-busy and the loading copy.
	let { class: className = '' }: { class?: string } = $props();
</script>

<span
	class="block rounded bg-line motion-safe:animate-pulse {className}"
	aria-hidden="true"
	data-skeleton
></span>
