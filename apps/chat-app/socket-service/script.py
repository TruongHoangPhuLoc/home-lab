import asyncio
import websockets
import json
import threading
# Dictionary to store WebSocket connections
connections = {}


async def handle_connection(websocket, path):
    try:
        async for message in websocket:
            # Handle incoming messages
            if message != "ping":
                data = json.loads(message)
                if data['type'] == 'Initialization':
                    # Assign identifier for each initialized connection
                    print("received new connections")
                    id = data['data']
                    connections[id] = websocket
                    print(connections)
                elif data['type'] == 'Accepted':
                    print(connections)      
                    recipient_id = data['sender_id']
                    recipient_conn = connections[recipient_id]
                    if recipient_conn:
                        sending_message = {'type': 'Reload', 'data': 'OK'}
                        await recipient_conn.send(json.dumps(sending_message))
                    else:
                        print("Cant find the connection")
                else:
                    print("Type is not standardlized yet")
            else:
                print(message)

    except websockets.exceptions.ConnectionClosedError:
        # Handle connection closed by client
        print("Connection closed by client")
    finally:
        # Clean up resources after the connection is closed
        print("Connection closed")
        # Remove the connection from the dictionary
        del connections[str(id)]
        print("Remaining connections: ", connections)
# async def keepConnectionsAlive():
#     while True:
#         print("Start KeepAlive Process")
#         listconenction = connections.values()
#         if len(listconenction) > 0:
#             print("Remaining connections ", connections)
#             for item in listconenction:
#                 await item.send(json.dumps({'type': 'Ping', 'data': 'OK'}))
#                 await asyncio.sleep(10)

async def main():
    # Start the WebSocket server
    async with websockets.serve(handle_connection, "0.0.0.0", 8000):
        print("WebSocket server started")
        # Keep the event loop running
        await asyncio.Future()
# Run the WebSocket server

# server = threading.Thread(target=asyncio.run, args=(main(),))
# server.start()
# keepalived = threading.Thread(target=asyncio.run, args=(keepConnectionsAlive(),))
# keepalived.start()
asyncio.run(main())
