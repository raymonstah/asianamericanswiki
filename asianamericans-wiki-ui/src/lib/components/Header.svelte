<script>
	import { getAuth, signOut } from 'firebase/auth';
	import { loggedIn } from './store.js';
	import { onDestroy } from 'svelte';

	let loggedInValue;
	const sub = loggedIn.subscribe((value) => {
		loggedInValue = value;
	});

	// unsubscribe when page goes away
	onDestroy(() => {
		sub();
	});

	function logout() {
		const auth = getAuth();
		console.log("logging out")
		signOut(auth)
			.then(() => {
				loggedIn.set(false);
			})
			.catch((error) => {
				console.log(error);
			});
	}
</script>

<header>
	<a href="/">Home</a>

	<nav>
		<ul>
			<li>
				<a href="/contribute">Contribute</a>
			</li>
			<li>
				<a href="/about">Our Story</a>
			</li>
			<li>
				<a href="/humans">Humans</a>
			</li>
			<li>
				<a href="/search">Search</a>
			</li>
			<li>
				{#if loggedInValue}
					<a href={'#'} on:click={logout}>Logout</a>
				{:else}
					<a href="/login">Login</a>
				{/if}
			</li>
		</ul>
	</nav>
</header>

<style>
	header {
		padding: 1rem;
		background: lightskyblue;
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
	}

	ul {
		margin: 0;
		list-style-type: none;
		display: flex;
		gap: 1rem;
	}

	a {
		text-decoration: none;
		color: inherit;
	}
</style>
