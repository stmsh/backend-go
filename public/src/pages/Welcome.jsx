import { useNavigate } from "@solidjs/router";

export function Welcome() {
    const navigate = useNavigate()

    return (
        <div>
            <form
                onSubmit={(event) => {
                    event.preventDefault()
                    const formData = new FormData(event.target)
                    localStorage.setItem("name", formData.get("name"));
                    navigate("/", { replace: true })
                }}
            >
                <input name="name" />
                <button>Next</button>
            </form>
        </div>
    );
}
