import asyncio
import websockets

async def connect_and_listen():
    uri = "ws://localhost:8000"  # Change the URI to match your WebSocket server
    try:
            async with websockets.connect(uri) as websocket:
                print("Connected to server")
                # Keep the connection alive and handle incoming messages
                try:
                    async for message in websocket:
                        print("Received message:", message)
                        # Add your message handling logic here
                except websockets.exceptions.ConnectionClosedError:
                    print("Connection closed by server")
    except Exception as e:
            print("Error:", e)
            # Wait for a short duration before attempting to reconnect
            await asyncio.sleep(5)

async def main():
    uri = "ws://localhost:8000"
    async with websockets.connect(uri) as websocket:
        await websocket.recv()
        

# Run the WebSocket client
asyncio.run(main())