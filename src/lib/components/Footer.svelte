<script>
  const year = new Date().getFullYear();
</script>

<footer>
  <a href={"#"}
    ><img src="/scalable.svg" alt="smaller logo" width="30" height="30" /></a
  >
  <span id="copyright">&copy; AsianAmericans.wiki {year}</span>
  <ul class="links">
    <li>
      <a
        href="https://github.com/raymonstah/asianamericanswiki/"
        target="_blank"
        ><img
          class="social"
          src="/github.svg"
          alt="github logo"
          width="32"
          height="32"
        /></a
      >
    </li>
    <li>
      <a href="https://twitter.com/aapiwiki" target="_blank"
        ><img
          class="social"
          src="/twitter.svg"
          alt="twitter logo"
          width="32"
          height="32"
        /></a
      >
    </li>
  </ul>
</footer>

<style>
  footer {
    /*pushes footer to bottom*/
    margin-top: auto;

    padding: 1rem 2rem;
    background: black;
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    justify-items: center;
    color: #eeeeee;
  }

  .social {
    filter: invert(100%) sepia(1%) saturate(269%) hue-rotate(175deg)
      brightness(116%) contrast(87%);
  }

  ul {
    margin: 0;
    list-style-type: none;
    display: flex;
    gap: 1rem;
  }

  .links {
    display: flex;
    justify-content: center;
    align-items: center;
  }

  a {
    text-decoration: none;
    color: inherit;
  }

  #copyright {
    display: flex;
    justify-content: center;
    align-items: center;
  }
</style>
