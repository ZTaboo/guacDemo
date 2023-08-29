import {useEffect, useLayoutEffect, useRef, useState} from 'react'
import Guacamole from 'guacamole-common-js'
import './App.css'


function App() {
    const [count, setCount] = useState(0)
    const initGuac = () => {
        const tunnel = new Guacamole.WebSocketTunnel("ws://localhost:8080/ws")
        const client = new Guacamole.Client(tunnel)
        const display = client.getDisplay()
        let displayElm = document.getElementById("views")
        const mouse = new Guacamole.Mouse(displayElm);
        const keyBoard = new Guacamole.Keyboard(displayElm)
        displayElm.appendChild(display.getElement())
        if (client) {
            keyBoard.onkeydown = keyBoard.onkeyup = () => {
            }
        }
        client.connect("")
        mouse.onEach(['mousedown', 'mousemove', 'mouseup'], function sendMouseEvent(e) {
            client.sendMouseState(e.state, true);
        });

        keyBoard.onkeydown = function (e) {
            console.log(e)
            client.sendKeyEvent(1, e)
        }
        keyBoard.onkeyup = function (e) {
            console.log(e)
            client.sendKeyEvent(0, e)
        }
    }
    useLayoutEffect(() => {
        initGuac()
    }, [initGuac])
    return (
        <div id={"views"} tabIndex={0}>
        </div>
    )
}

export default App
