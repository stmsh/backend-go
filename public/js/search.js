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
    index = -1;

    constructor() {
        super();
    }

    connectedCallback() {
        this.classList.add("group");
        this.innerHTML = this.view();

        this.input = this.querySelector("#search_input");
        this.results = this.querySelector("#search_results");

        this.querySelector("#search_toggle").addEventListener("click", () => {
            if (this.hasAttribute("data-expanded")) {
                this.removeAttribute("data-expanded");
                this.index = -1;
            } else {
                this.input.focus();
            }
        });
        this.querySelector("#search_clear").addEventListener("click", () => {
            this.input.value = "";
            this.input.focus();
        });

        this.input.addEventListener("keydown", this.onKeyDown);
        this.input.addEventListener("input", this.onInput);
        this.input.addEventListener("focus", () => {
            this.setAttribute("data-expanded", "true");
            this.results.scrollTo({ top: 0 });
        });

        this.results.addEventListener("click", (e) => {
            const li = e.target.closest("li");
            const index = Array.prototype.indexOf.call(
                this.results.children,
                li,
            );
            this.index = index;
            this.updateOptionHighlight();

            if (li.hasAttribute("data-selected")) {
                li.removeAttribute("data-selected");
            } else {
                li.setAttribute("data-selected", "true");
            }
        });
    }

    disconnectedCallback() {}

    updateIndex(delta) {
        const n = this.results.childElementCount;
        this.index = (this.index + delta + n) % n;
    }

    /** @param {KeyboardEvent} e */
    onKeyDown = (e) => {
        switch (e.key) {
            case "ArrowDown":
                e.preventDefault();
                this.updateIndex(1);
                this.updateOptionHighlight();
                break;

            case "ArrowUp":
                e.preventDefault();
                this.updateIndex(-1);
                this.updateOptionHighlight();
                break;

            case "Escape":
                e.preventDefault();
                this.removeAttribute("data-expanded");
                this.input.blur();
                this.index = -1;
                this.updateOptionHighlight();
                break;

            case "Enter":
                e.preventDefault();
                const options = Array.from(this.results.children);
                options[this.index]?.click();
                break;
        }
    };

    onInput = debounce(
        /** @param {InputEvent} e  */
        (e) => {
            this.index = -1;
            this.results.scrollTo({ top: 0 });
            fetch(`/search?query=${e.target.value}`)
                .then((r) => r.json())
                .then((r) => this.updateResults(r.results));
        },
        500,
    );

    /** @param {HTMLElement} option */
    selectOption(option) {
        option.setAttribute("data-selected", "true");
    }

    /** @param {HTMLElement} option */
    isOptionInView(option) {
        const bounds = option.getBoundingClientRect();
        return (
            bounds.top >= 0 &&
            bounds.left >= 0 &&
            bounds.bottom <= document.documentElement.clientHeight &&
            bounds.right <= document.documentElement.clientWidth
        );
    }

    /** @param {ResultEntry[]} results */
    updateResults(results) {
        this.results.innerHTML = results.map(this.viewItem).join("");
        htmx.process(this.results);
    }

    updateOptionHighlight() {
        const options = Array.from(this.results.children);
        const currentlyActive = this.results.querySelector(
            "[data-active='true']",
        );
        currentlyActive?.removeAttribute("data-active");

        if (this.index === -1) {
            return;
        }

        const active = options[this.index];
        active.setAttribute("data-active", "true");

        if (!this.isOptionInView(active)) {
            active.scrollIntoView({ block: "end" });
        }
    }

    /** @param {ResultEntry} r */
    viewItem(r) {
        const valsAdd = JSON.stringify({
            type: "list_add",
            payload: r,
        });
        const valsRemove = JSON.stringify({
            type: "list_remove",
            payload: { id: String(r.id) },
        });

        const posterUrl = r.poster_path
            ? `https://image.tmdb.org/t/p/w500/${r.poster_path}`
            : "/public/no_poster.svg";

        return `
<li 
    ws-send 
    hx-trigger="click" 
    hx-vals='js:...(event.target.hasAttribute("data-selected") ? ${escape(valsRemove)}: ${escape(valsAdd)})' 
    class="flex gap-2 cursor-pointer data-[active='true']:outline data-[selected='true']:bg-green-100"
>
    <img
        src="${posterUrl}"
        alt="Poster to ${r.title}"
        class="aspect-[2/3] w-20 min-w-20"
    />
    <div>
        <p>${r.title}</p>
        <p>${r.release_date.slice(0, 4)}</p>
    </div>
</li>`;
    }

    view() {
        return `
<div class="
    flex
    flex-col
    min-h-0
    bg-white
    group-data-[expanded='true']:fixed
    group-data-[expanded='true']:top-0
    group-data-[expanded='true']:bottom-0
    group-data-[expanded='true']:left-0
    group-data-[expanded='true']:right-0
    "
>
    <div class="flex items-center gap-2 p-2 group-data-[expanded='true']:border-b-4">
        <button 
            id="search_toggle" 
            tabindex="-1"
            class="p-3"
        >
            <div class="
                bg-[url(/public/search.svg)]
                group-data-[expanded='true']:bg-[url(/public/arrow_left.svg)]
                w-[1lh] h-[1lh]
                bg-contain
                "
            ></div>
        </button>
        <input 
            id="search_input" 
            type="text" 
            autocomplete="off"
            placeholder="Search movies..."
            class="grow p-2 outline-none"
        >
        <button 
            id="search_clear" 
            class="hidden group-data-[expanded='true']:block p-3"
        >
            <div class="bg-[url(/public/cross.svg)] w-[1lh] h-[1lh] bg-contain"></div>
        </button>
    </div>
    <ul
        id="search_results" 
        class="
            hidden
            group-data-[expanded='true']:flex
            flex-col
            gap-2
            p-2
            overflow-y-auto
            "
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

customElements.define("movie-search", Search);
