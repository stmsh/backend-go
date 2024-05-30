import { onMount } from "solid-js";

export function Room(props) {
    const socket = new WebSocket("ws://localhost:8080/ws");

    onMount(() => {
        socket.onopen = () => {
            socket.send(
                JSON.stringify(
                    {
                        type: "init",
                        payload: {
                            name: localStorage.getItem("name"),
                            roomid: props.params.id,
                        },
                    },
                    document.body
                )
            );
        };

        socket.onmessage = (_, event) => {
            console.log(event);
        };
    });

    return <div>Room #{props.params.id}</div>;
}
