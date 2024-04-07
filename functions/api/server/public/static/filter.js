const urlParams = new URLSearchParams(window.location.search);

const ethnicity = urlParams.get("ethnicity");
const tag = urlParams.get("tag");
const gender = urlParams.get("gender");
const search = urlParams.get("search");

var ethnicitySelected = document.getElementById("ethnicity");
if (ethnicity != null) {
  ethnicitySelected.value = ethnicity;
}

var genderSelected = document.getElementById("gender");
if (gender != null) {
  genderSelected.value = gender;
}

var dobBefore = urlParams.get("dobBefore");
var dobAfter = urlParams.get("dobAfter");

if (dobBefore != null) {
  let minAge = convertYYYYMMDDToAge(dobBefore);
  console.log("minAge", minAge);
  var minAgeSelected = document.getElementById("minAge");
  minAgeSelected.value = minAge;
}
if (dobAfter != null) {
  let maxAge = convertYYYYMMDDToAge(dobAfter);
  console.log("maxAge", maxAge);
  var maxAgeSelected = document.getElementById("maxAge");
  maxAgeSelected.value = maxAge;
}
if (tag != null) {
  var tagSelected = document.getElementById("tags");
  tagSelected.value = tag;
}

var searchInput = document.getElementById("search");
if (search != null) {
  searchInput.value = search;
}

function removeEmpty(obj) {
  return Object.fromEntries(Object.entries(obj).filter(([_, v]) => v != ""));
}

function doSearch() {
  var minAgeSelected = document.getElementById("minAge");
  var maxAgeSelected = document.getElementById("maxAge");
  var tagSelected = document.getElementById("tags");
  const params = new URLSearchParams(
    removeEmpty({
      dobBefore: convertToYYYYMMDDString(minAgeSelected.value),
      dobAfter: convertToYYYYMMDDString(maxAgeSelected.value),
      gender: genderSelected.value,
      ethnicity: ethnicitySelected.value,
      tag: tagSelected.value,
      search: searchInput.value,
    }),
  );
  console.log("search parameters", params.toString());
  window.location.href = "/humans/?" + params.toString();
}

function convertToYYYYMMDDString(year) {
  if (year === "") {
    return "";
  }
  const currentDate = new Date();
  const targetDate = new Date(currentDate.getFullYear() - year, 0, 1);

  // Extracting YYYY-MM-DD format
  const yearString = targetDate.getFullYear();
  const monthString = (targetDate.getMonth() + 1).toString().padStart(2, "0");
  const dayString = targetDate.getDate().toString().padStart(2, "0");

  return `${yearString}-${monthString}-${dayString}`;
}

function convertYYYYMMDDToAge(birthDate) {
  const currentDate = new Date();
  const birthDateObject = new Date(birthDate);
  const age = currentDate.getFullYear() - birthDateObject.getFullYear();
  const monthDifference = currentDate.getMonth() - birthDateObject.getMonth();
  if (
    monthDifference < 0 ||
    (monthDifference === 0 && currentDate.getDate() < birthDateObject.getDate())
  ) {
    return age - 1;
  }
  return age;
}
