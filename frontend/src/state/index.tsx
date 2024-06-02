interface Player {
    id: string;
    name: string;
    ready: boolean;
    isHost: boolean;
}

interface ListItem {
    id: string;
    title: string;
    overview: string;
    rating: string;
    release_date: string;
    poster_path: string;
}

interface Candidate {
    id: string;
    name: string;
    suggestedBy: string;
}

interface State {
    me: Player

}
