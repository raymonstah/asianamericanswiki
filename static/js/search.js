// taken from https://victoria.dev/blog/add-search-to-hugo-static-sites-with-lunr/
function displayResults(results, store) {
  const searchResults = document.getElementById("results");
  if (results.length) {
    let resultList = "";
    // Iterate and build result list elements
    for (const n in results) {
      const item = store[results[n].ref];
      resultList +=
        '<div class="bg-white mv3 pa4 gray overflow-hidden"><h1 class="f3 near-black"><a class="link black dim" href="' +
        item.url +
        '">' +
        item.title +
        "</a></h1>";
      resultList +=
        '<p class="nested-links f5 lh-copy nested-copy-line-height">' +
        item.content.substring(0, 150) +
        "...</p></div>";
    }
    searchResults.innerHTML = resultList;
  } else {
    searchResults.innerHTML =
      "No results found -- consider making a <a href='/contribute'>contribution</a>.";
  }
}

// Get the query parameter(s)
const params = new URLSearchParams(window.location.search);
const query = params.get("query");
window.onload = function () {
  // Perform a search if there is a query
  if (query) {
    // Retain the search input in the form when displaying results
    document.getElementById("search-input").setAttribute("value", query);
    const idx = lunr(function () {
      this.ref("id");
      this.field("title", {
        boost: 15,
      });
      this.field("tags");
      this.field("ethnicity");
      this.field("content", {
        boost: 10,
      });

      for (const key in humansSearchIndex) {
        this.add({
          id: key,
          title: humansSearchIndex[key].title,
          tags: humansSearchIndex[key].tags,
          content: humansSearchIndex[key].content,
          ethnicity: humansSearchIndex[key].ethnicity,
        });
      }
    });

    // Perform the search
    const results = idx.search(query);
    // Update the list with results
    displayResults(results, humansSearchIndex);
  }
};
