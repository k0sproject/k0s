window.addEventListener("DOMContentLoaded", function() {
  // This is a bit hacky. Figure out the base URL from a known CSS file the
  // template refers to...
  var ex = new RegExp("/?css/version-select.css$");
  var sheet = document.querySelector('link[href$="version-select.css"]');

  var ABS_BASE_URL = sheet.href.replace(ex, "");
  var CURRENT_VERSION = ABS_BASE_URL.split("/").pop();

  function makeSelect(options, selected) {
    var select = document.createElement("select");
    select.classList.add("form-control");

    options.forEach(function(i) {
      var option = new Option(i.text, i.value, undefined,
                              i.value === selected);
      select.add(option);
    });

    return select;
  }

  var xhr = new XMLHttpRequest();
  xhr.open("GET", ABS_BASE_URL + "/../versions.json");
  xhr.onload = function() {
    var versions = JSON.parse(this.responseText);

    var realVersion = versions.find(function(i) {
      return i.version === CURRENT_VERSION ||
             i.aliases.includes(CURRENT_VERSION);
    }).version;

    var select = makeSelect(versions.map(function(i) {
      return {text: i.title, value: i.version};
    }), realVersion);
    select.addEventListener("change", function(event) {
      window.location.href = ABS_BASE_URL + "/../" + this.value;
    });

    var container = document.createElement("div");
    container.id = "version-selector";
    container.className = "md-nav__item";
    container.appendChild(select);

    var sidebar = document.querySelector(".md-nav--primary > .md-nav__list");
    sidebar.parentNode.insertBefore(container, sidebar);
  };
  xhr.send();
});
