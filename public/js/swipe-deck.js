const SWIPE_DIRECTIONS = {
    left: "left",
    right: "right",
};

const DEFAULT_SWIPE_DISTANCE = 100;
const DEFAULT_ANGLE = 15;

class SwipeDeck extends HTMLElement {
    static observedAttributes = ["swipe-distance"];

    /** @type HTMLElement | null */
    focusedElement;

    startX = 0;
    startY = 0;

    constructor() {
        super();
    }

    getSwipeDistance() {
        return (
            Number(this.getAttribute("swipe-distance")) ||
            DEFAULT_SWIPE_DISTANCE
        );
    }

    connectedCallback() {
        const children = Array.from(this.children);
        children.forEach((child) => this.setup(child));
        document.addEventListener("pointermove", this.handleMove);

        const styles = document.createElement("style");
        styles.innerHTML = `
.swipe_deck__item {
    --swipe-deck-dx: 0px;
    --swipe-deck-angle: 0deg;
    --swipe-deck-progress: 0;

    cursor: grab;
    user-select: none;
    transform-origin: center;
    touch-action: none;
    transform: translateX(var(--swipe-deck-dx)) rotate(var(--swipe-deck-angle));
}

.swipe_deck__item--dragging {
    cursor: grabbing;
    transition: none;
}

.swipe_deck__item--releasing {
    transition: transform .1s;
}

.swipe_deck__item--left::after {
    content: '👎';
    font-size: 4rem;
    position: absolute;
    top: 1rem;
    right: 1rem;

    display: block;
    width: 6rem;
    height: 6rem;
    border-radius: 50%;
    line-height: 7rem;
    text-align: center;

    background-image:   
        conic-gradient(transparent calc(100% - var(--swipe-deck-progress) * 100%), white 0);
}

.swipe_deck__item--right::after {
    content: '👍';
    font-size: 4rem;
    position: absolute;
    top: 1rem;
    left: 1rem;

    display: block;
    width: 6rem;
    height: 6rem;
    border-radius: 50%;
    text-align: center;

    background-image: 
        conic-gradient(white calc(var(--swipe-deck-progress) * 100%), transparent 0);
}

.swipe_deck__item--left::before {
    position: absolute;
    display: block;
    content: '';
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background-color: #f87171;
    opacity: calc(var(--swipe-deck-progress) * 0.8);
    border-radius: inherit;
}

.swipe_deck__item--right::before {
    position: absolute;
    display: block;
    content: '';
    top: 0;
    right: 0;
    width: 100%;
    height: 100%;
    background-color: #4ade80;
    opacity: calc(var(--swipe-deck-progress) * 0.8);
    border-radius: inherit;
}
`;
        document.head.insertAdjacentElement("beforeend", styles);
    }

    disconnectedCallback() {
        const children = Array.from(this.children);
        children.forEach((child) => this.cleanup(child));
        document.removeEventListener("pointermove", this.handleMove);
    }

    /** @param {HTMLElement} el */
    setup(el) {
        el.classList.add("swipe_deck__item");
        el.addEventListener("pointerdown", this.handleMoveStart);
    }

    /** @param {HTMLElement} el */
    cleanup(el) {
        el.removeEventListener("pointerdown", this.handleMoveStart);
    }

    /** @param {PointerEvent} e */
    handleMoveStart = (e) => {
        e.stopPropagation();

        this.focusedElement = e.currentTarget;
        this.focusedElement.classList.add("swipe_deck__item--dragging");
        this.startX = e.clientX;
        this.startY = e.clientY;

        document.addEventListener("pointerup", this.handleRelease, {
            once: true,
        });
    };

    /** @param {PointerEvent} e */
    handleMove = (e) => {
        if (!this.focusedElement) {
            return;
        }
        e.stopPropagation();

        const distance = this.getSwipeDistance();
        const dx = clamp(-distance, e.clientX - this.startX, distance);
        const progress = Math.abs(dx / distance);
        const angle = Math.sign(dx) * progress * DEFAULT_ANGLE;

        this.focusedElement.style.setProperty("--swipe-deck-dx", dx + "px");
        this.focusedElement.style.setProperty(
            "--swipe-deck-progress",
            progress,
        );
        this.focusedElement.style.setProperty(
            "--swipe-deck-angle",
            angle + "deg",
        );
        this.focusedElement.classList.add("swipe_deck__item--dragging");

        if (dx > 0) {
            this.focusedElement.classList.remove("swipe_deck__item--left");
            this.focusedElement.classList.add("swipe_deck__item--right");
        } else {
            this.focusedElement.classList.add("swipe_deck__item--left");
            this.focusedElement.classList.remove("swipe_deck__item--right");
        }
    };

    /** @param {PointerEvent} e */
    handleRelease = (e) => {
        e.stopPropagation();
        const target = this.focusedElement;
        this.focusedElement = null;

        target.classList.remove("swipe_deck__item--dragging");

        const moveDistance = e.clientX - this.startX;
        if (Math.abs(moveDistance) < this.getSwipeDistance()) {
            target.style.removeProperty("--swipe-deck-dx");
            target.style.removeProperty("--swipe-deck-angle");
            target.style.removeProperty("--swipe-deck-progress");
            target.classList.remove(
                "swipe_deck__item--left",
                "swipe_deck__item--right",
            );
            target.classList.add("swipe_deck__item--releasing");

            target.addEventListener(
                "transitionend",
                () => target.classList.remove("swipe_deck__item--releasing"),
                { once: true },
            );
            return;
        }

        const direction =
            moveDistance > 0 ? SWIPE_DIRECTIONS.right : SWIPE_DIRECTIONS.left;

        this.dispatchEvent(
            new CustomEvent("swipe", {
                detail: { target, direction },
            }),
        );
        target.remove();
    };
}

function clamp(min, v, max) {
    return Math.min(Math.max(min, v), max);
}

customElements.define("swipe-deck", SwipeDeck);
