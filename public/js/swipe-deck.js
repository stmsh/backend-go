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
    }

    disconnectedCallback() {
        const children = Array.from(this.children);
        children.forEach((child) => this.cleanup(child));
        document.removeEventListener("pointermove", this.handleMove);
    }

    /** @param {HTMLElement} el */
    setup(el) {
        el.style.cursor = "grab";
        el.style.userSelect = "none";
        el.style.transformOrigin = "center";
        el.style.touchAction = "none";
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
        this.focusedElement.style.cursor = "grabbing";
        this.focusedElement.style.transition = "";
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

        const swipeDistance = this.getSwipeDistance();
        const dx = clamp(
            -swipeDistance,
            e.clientX - this.startX,
            swipeDistance,
        );
        const swipeRatio = dx / swipeDistance;
        const angle = swipeRatio * DEFAULT_ANGLE;

        this.focusedElement.style.transform = `translateX(${dx}px) rotate(${angle}deg)`;
    };

    /** @param {PointerEvent} e */
    handleRelease = (e) => {
        e.stopPropagation();
        const target = this.focusedElement;
        target.style.cursor = "grab";
        this.focusedElement = null;

        const moveDistance = e.clientX - this.startX;
        if (Math.abs(moveDistance) < this.getSwipeDistance()) {
            target.style.transition = "transform .1s";
            target.style.transform = "";
            target.addEventListener(
                "transitionend",
                () => {
                    target.style.transition = "";
                },
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
