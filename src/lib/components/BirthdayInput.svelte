<script>
  const DATE_FORMAT = {
    "yyyy-mm-dd": new RegExp("^\\d{4}-\\d{2}-\\d{2}$"),
    "mm/dd": new RegExp("^\\d{1,2}\\/\\d{1,2}$"),
    "mm/dd/yyyy": new RegExp("^\\d{1,2}\\/\\d{1,2}\\/\\d{4}$"),
    yyyy: new RegExp("^\\d{4}$"),
  };

  export let dob = "";
  export let invalid = false;
  function validate() {
    if (dob.length === 0) {
      invalid = false;
      return;
    }
    let match = false;
    Object.entries(DATE_FORMAT).find((entry) => {
      let format = entry[0];
      let regex = entry[1];
      // console.log(`testing ${dob} against ${format}`);
      if (regex.test(dob)) {
        // console.log(`${format} matches`);
        match = true;
      }
    });
    invalid = !match;
  }
</script>

<input
  class={`${$$props.class}`}
  type="text"
  minlength="4"
  maxlength="10"
  class:dob-invalid={invalid}
  placeholder="yyyy-MM-dd or MM/dd or MM/dd/yyyy or yyyy"
  on:keyup={validate}
  bind:value={dob}
/>

<style>
  .dob-invalid {
    border-color: red;
  }
</style>
