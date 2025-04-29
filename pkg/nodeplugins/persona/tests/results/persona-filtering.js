function isMobileDevice() {
    return window.innerWidth <= 768;
}

function createDropdown() {
    const navItems = document.querySelectorAll('.nav-item');
    if (navItems.length == 6 && !isMobileDevice()) {
        const dropdownMenu = document.createElement('div');
        dropdownMenu.className = 'dropdown-content';

        const dropdownItems = [
            { text: 'Users', href: '/docs' },
            { text: 'Operators', href: '/docs' },
            { text: 'Developers', href: '/docs' },
            { text: 'All', href: '/docs' }
        ];

        dropdownItems.forEach(item => {
            const a = document.createElement('a');
            a.className = 'taxonomy-term';
            a.href = item.href;
            a.textContent = item.text;

            a.addEventListener('click', function(event) {
                const allItems = dropdownMenu.querySelectorAll('.taxonomy-term');
                allItems.forEach(item => item.classList.remove('selectedTaxonomy'));
                this.classList.add('selectedTaxonomy');
            });

            dropdownMenu.appendChild(a);
        });

        navItems[2].appendChild(dropdownMenu);

        navItems[2].classList.add('dropdown');
        const navLink = navItems[2].querySelector('.nav-link');
        navLink.classList.add('dropdown-toggle');
        navLink.setAttribute('data-toggle', 'dropdown');
        navLink.setAttribute('aria-haspopup', 'true');
        navLink.setAttribute('aria-expanded', 'false');
    }
}

createDropdown();

const currentPath = window.location.pathname;

const searchString = "/docs/";
if (currentPath.includes(searchString)){

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

linkToPersona = {
    "/docs/": "Developers,Users,Operators",
    "/docs/foo/": "Operators",
    "/docs/getting-started/": "Developers,Users",
    "/docs/getting-started/users-content/": "Users",
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

}