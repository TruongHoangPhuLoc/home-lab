import socket

def start_server(host='0.0.0.0', port=65432):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as server_socket:
        server_socket.bind((host, port))
        server_socket.listen()
        print(f'Serving on {host}:{port}')

        while True:
            conn, addr = server_socket.accept()
            with conn:
                print(f'Connected by {addr}')
                while True:
                    try:
                        data = conn.recv(1024)
                        if not data:
                            print(f'Connection closed by {addr}')
                            break
                        # Handle received data
                        conn.sendall(data)  # Echo received data back to client
                    except ConnectionResetError:
                        print(f'Connection reset by {addr}')
                        break

if __name__ == "__main__":
    start_server()