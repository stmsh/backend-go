import { Navigate, Route, Router } from "@solidjs/router";
import { Home } from "./pages/Home"
import { Room } from "./pages/Room"
import { Welcome } from "./pages/Welcome";

const App = () => {
    return (
        <Router>
            <Route path="/" component={Home} />
            <Route path="/welcome" component={Welcome} />
            <Route path="/rooms/:id" component={Room} />
            <Route path="*" component={() => <Navigate href="/" />} />
        </Router>
    );
};

export default App;
