<script>
  export let url;
  export let name;
  export let description;
</script>

<!-- Affiliate Link Display Component -->
<div class="text-center">
  <h2 class="text-xl font-semibold mb-4"><a href={url}>{name}</a></h2>
  <!-- <img src="product_image.jpg" alt="Product" class="w-64 mx-auto mb-4" /> -->
  <!-- <p class="text-gray-600 text-sm mb-4">
    {description}
  </p> -->
  <div class="flex justify-center space-x-4">
    <a
      href={url}
      class="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded transition duration-300 ease-in-out"
      >Buy Now on Amazon</a
    >
  </div>
</div>
<!-- End of Affiliate Link Display Component -->
