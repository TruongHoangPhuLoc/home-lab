import asyncio
import websockets

async def handle_connection(websocket, path):
    try:
        print("Connected to client")
        await websocket.send("Hello")
    except websockets.exceptions.ConnectionClosedError:
        print("Error")
    finally:
        print("Connection closed")
async def main():
    # Start the WebSocket server
    async with websockets.serve(handle_connection, "0.0.0.0", 8000, ping_interval=20):
        print("WebSocket server started")
        # Keep the event loop running
        await asyncio.Future()

# Run the WebSocket server
asyncio.run(main())
