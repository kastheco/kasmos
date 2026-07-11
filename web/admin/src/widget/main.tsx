import React from "react";
import { createRoot } from "react-dom/client";
import "../globals.css";
import Monitor from "./Monitor";

const element = document.getElementById("root");
if (element) createRoot(element).render(<React.StrictMode><Monitor /></React.StrictMode>);
