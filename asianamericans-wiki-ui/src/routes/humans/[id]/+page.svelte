<script>
	export let data;
</script>

<article>
	<h1>{data.metadata.title}</h1>

	<p>Ethnicity: {data.metadata.ethnicity}</p>
	<p>Published: {data.metadata.date}</p>

	{@html data.content}
</article>
