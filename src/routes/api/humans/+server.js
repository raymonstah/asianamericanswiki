import { error } from "@sveltejs/kit";

export const GET = async ({ url }) => {
  try {
    /**
     * These let you add query params to change what's retrieved from the endpoint, e.g.,
     * /api/posts.json?offset=10&limit=20
     **/
    const params = new URLSearchParams(url.search);

    const options = {
      offset: parseInt(params.get("offset")) || null,
      limit: parseInt(params.get("limit")) || -1,
      tags: params.getAll("tags") || [],
      ethnicity: params.getAll("ethnicity") || [],
    };

    const posts = await fetchMarkdownPosts(options);
    return new Response(JSON.stringify(posts), {
      status: 200,
      headers: {
        "content-type": "application/json",
      },
    });
  } catch (err) {
    throw error(500, `Could not fetch humans. ${err}`);
  }
};

const fetchMarkdownPosts = async ({
  offset = 0,
  limit = -1,
  tags = [],
  ethnicity = [],
} = {}) => {
  const allPostFiles = import.meta.glob("/content/humans/**/index.md");
  const iterablePostFiles = Object.entries(allPostFiles);
  let allPosts = await Promise.all(
    iterablePostFiles.map(async ([path, resolver]) => {
      const { metadata } = await resolver();
      const postPath = path.slice(9, -9);
      return {
        meta: metadata,
        path: "/" + postPath,
      };
    })
  );

  // tags OR'ed, tagA or tagB or tagC...
  if (tags.length) {
    allPosts = allPosts.filter((post) => {
      return post.meta.tags.some((t) => tags.includes(t.toLowerCase()));
    });
  }

  if (ethnicity.length) {
    allPosts = allPosts.filter((post) => {
      return post.meta.ethnicity.some((e) =>
        ethnicity.includes(e.toLowerCase())
      );
    });
  }

  if (offset) {
    allPosts = allPosts.slice(offset);
  }

  if (limit && limit < allPosts.length && limit !== -1) {
    allPosts = allPosts.slice(0, limit);
  }

  return allPosts;
};
