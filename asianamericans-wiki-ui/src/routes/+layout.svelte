<script>
	import Header from '$lib/components/Header.svelte';
	import '$lib/styles/style.css';
	import Auth from '$lib/components/Auth.svelte';
</script>

<Auth />
<Header />

<main>
	<slot />
</main>

<footer>Hello, I'm the footer.</footer>
