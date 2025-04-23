document.querySelectorAll(".taxonomy-term").forEach((el) => {
    el.addEventListener("click",(event) => {
      const roleSelected = event.currentTarget.innerHTML
      window.sessionStorage.setItem("role_selected",roleSelected)
      location.reload()
    })
})
const taxonomyTerms = Array.from(document.querySelectorAll(".taxonomy-term"))
taxonomyTerms
    .filter(tt => tt.innerHTML.includes(window.sessionStorage.getItem("role_selected")))
    .map(tt => tt.setAttribute("class","taxonomy-term selectedTaxonomy"))

linkToPersona = { {{- range $key, $value := . }}
    "{{ $key }}": "{{ $value }}",{{- end }}
}

const selectedPersona = window.sessionStorage.getItem("role_selected")

function shouldHide(href) {
    const personas = linkToPersona[href];
    // If there are no personas associated with the href, do not hide
    if (!personas) {
        return false;
    }
    // If the selected persona is null or "All", do not hide
    if (selectedPersona == null || selectedPersona === "All") {
        return false;
    }
    // Hide if the personas do not include the selected persona
    return !personas.includes(selectedPersona);
}

Array.from(document.querySelectorAll(".entry")).filter(el => {
    const href = el.querySelector("a").getAttribute("href");
    return shouldHide(href);
}).forEach(el => el.style.display = "none");

Array.from(document.querySelectorAll(".td-sidebar-nav__section")).filter(el => {
    const href = el.querySelector("a").getAttribute("href");    
    return shouldHide(href);
}).forEach(bel => bel.style.display = "none");