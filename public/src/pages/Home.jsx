import { Navigate } from "@solidjs/router";
import { Match, Switch } from "solid-js";

export function Home(props) {
    const name = localStorage.getItem("name");

    return (
        <Switch>
            <Match when={!name}>
                <Navigate href="/welcome" />
            </Match>

            <Match when={true}>
                <p>Ask your friend for a link</p>
                <p>Or</p>
                <p><button>Create</button> your own room</p>
            </Match>
        </Switch>
    );
}
