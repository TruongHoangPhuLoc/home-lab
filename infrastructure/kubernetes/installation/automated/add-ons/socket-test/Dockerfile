# Use the official Python image from the Docker Hub
FROM python:3.9-slim

# Set the working directory
WORKDIR /app

# Copy the TCP server script into the container
COPY tcp_server.py /app

# Expose the port the application will run on
EXPOSE 65432

# Define the command to run the application
CMD ["python3", "-u", "tcp_server.py"]
