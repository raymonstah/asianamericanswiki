<script>
  export let url;
  export let name;
  export let image;
  export let description;
</script>

<!-- Affiliate Link Display Component -->
<div class="text-center flex flex-col h-full">
  <div class="flex-grow">
    <h2 class="text-xl font-semibold mb-4">
      <a href={url} target="_blank">{name}</a>
    </h2>
    <!-- if image is not empty -->
    {#if image}
      <a href={url} target="_blank"
        ><img src={image} alt={name} class="w-64 mx-auto mb-4" /></a
      >
    {/if}
    <!-- <p class="text-gray-600 text-sm mb-4">
        {description}
      </p> -->
  </div>
  <div class="flex justify-center">
    <a
      href={url}
      target="_blank"
      class="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded transition duration-300 ease-in-out"
      >Buy Now on Amazon</a
    >
  </div>
</div>
<!-- End of Affiliate Link Display Component -->
