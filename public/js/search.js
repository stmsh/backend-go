/**
 * @typedef ResultEntry
 * @prop {string} id
 * @prop {string} title
 * @prop {string} release_date
 * @prop {string} overview
 * @prop {number} rating
 * @prop {string} poster_path
 */

class Search extends HTMLElement {
    /** @type {HTMLInputElement} */
    input;
    /** @type {HTMLUListElement} */
    results;

    constructor() {
        super();
    }

    connectedCallback() {
        this.innerHTML = this.view();
        this.input = this.querySelector("#search_input");
        this.results = this.querySelector("#search_results");

        this.input.addEventListener("keydown", this.onKeyDown);
    }

    disconnectedCallback() {
        this.input.removeEventListener("keydown", this.onKeyDown);
    }

    onKeyDown = debounce(
        /** @param {KeyboardEvent} e */
        (e) => {
            const query = e.target.value;
            fetch(`/search?query=${query}`)
                .then((r) => r.json())
                .then((r) => this.updateResults(r.results));
        },
        100,
    );

    /** @param {ResultEntry[]} results */
    updateResults(results) {
        this.results.innerHTML = results.map(this.viewItem).join("");
        htmx.process(this.results);
    }

    /** @param {ResultEntry} r */
    viewItem(r) {
        const hxVals = JSON.stringify({
            type: "list_add",
            payload: r,
        });

        const posterUrl = r.poster_path
            ? `https://image.tmdb.org/t/p/w500/${r.poster_path}`
            : "/public/no_poster.svg";

        return `
<li 
    tabIndex="0" 
    ws-send 
    hx-trigger="click,keydown[keyCode==13]" 
    hx-vals='js:${escape(hxVals)}' 
    class="flex"
>
    <img
        src="${posterUrl}"
        alt="Poster to ${r.title}"
        class="aspect-[2/3] w-20 min-w-20"
    />
    <div>
        <p>${r.title}</p>
        <p>${r.release_date}</p>
    </div>
</li>`;
    }

    view() {
        return `
<div class="
    group
    flex
    flex-col
    focus-within:absolute
    focus-within:top-0
    focus-within:bottom-0
    focus-within:left-0
    focus-within:right-0
    overflow-hidden
"
>
    <div class="flex items-center">
        ${svg}
        <input id="search_input" 
            type="search" 
            placeholder="Search movies..."
            class="grow"
        >
    </div>
    <ul
        id="search_results" 
        class="bg-white flex-grow flex-col hidden group-focus-within:flex overflow-y-auto"
    ></ul>
</div>
`;
    }
}

function debounce(fn, timeout) {
    let timeoutID;
    return function (...args) {
        if (timeoutID) {
            clearTimeout(timeoutID);
        }

        timeoutID = setTimeout(() => fn.apply(null, args), timeout);
    };
}

const htmlEscapeTable = {
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
};

function escape(value) {
    return value.replace(/[&<>"']/g, (ch) => htmlEscapeTable[ch]);
}

const svg = `
<svg
    version="1.1"
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 49.29974328242838 48.77927647211254"
    width="49.29974328242838"
    height="48.77927647211254"
>
    <rect
        x="0"
        y="0"
        width="49.29974328242838"
        height="48.77927647211254"
        fill="#ffffff"
    ></rect>
    <g
        stroke-linecap="round"
        transform="translate(10 9.999999999998181) rotate(0 10.82062977900307 10.820629779002957)"
    >
        <path
            d="M12.07 -0.39 C14.37 -0.39, 17.08 1.22, 18.65 2.85 C20.21 4.47, 21.32 6.93, 21.44 9.35 C21.57 11.77, 20.76 15.42, 19.4 17.35 C18.04 19.28, 15.53 20.41, 13.28 20.94 C11.03 21.47, 7.86 21.42, 5.88 20.51 C3.9 19.61, 2.31 17.58, 1.41 15.53 C0.51 13.48, -0.05 10.42, 0.5 8.22 C1.06 6.02, 2.76 3.64, 4.74 2.33 C6.71 1.01, 11.08 0.71, 12.36 0.33 C13.63 -0.05, 12.47 0, 12.38 0.06 M10.63 -0.6 C12.81 -0.89, 16.04 0.75, 17.71 2.27 C19.39 3.78, 20.32 6.23, 20.68 8.48 C21.03 10.73, 20.8 13.71, 19.84 15.76 C18.89 17.81, 16.97 19.79, 14.94 20.79 C12.91 21.78, 9.87 22.55, 7.67 21.75 C5.48 20.94, 3.11 17.93, 1.78 15.96 C0.44 13.98, -0.69 12.1, -0.32 9.89 C0.04 7.67, 2.11 4.24, 3.97 2.68 C5.83 1.12, 9.66 0.86, 10.82 0.54 C11.99 0.22, 10.98 0.73, 10.95 0.78"
            stroke="#1e1e1e"
            stroke-width="1"
            fill="none"
        ></path>
    </g>
    <g stroke-linecap="round">
        <g
            transform="translate(27.45540971220271 27.31558267365108) rotate(0 5.922166785112836 5.731846899230732)"
        >
            <path
                d="M0 0 C3.23 3.55, 8.19 6.61, 11.84 11.46 M0 0 C3.76 3.74, 8.57 7.66, 11.84 11.46"
                stroke="#1e1e1e"
                stroke-width="1"
                fill="none"
            ></path>
        </g>
    </g>
</svg>
`;

customElements.define("movie-search", Search);
