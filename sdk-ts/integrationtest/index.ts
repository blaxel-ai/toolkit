import { instrumentApp } from "../src/common/instrumentation";

instrumentApp().then(() => {
    console.log("Instrumentation initialized");
});

