document.addEventListener('DOMContentLoaded', function() {
    const addFriendForm = document.getElementById('addFriendForm');
    const messageDiv = document.getElementById('message');   
    addFriendForm.addEventListener('submit', function(event) {
        event.preventDefault();
        const friendUsername = document.getElementById('friendUsername').value;

        fetch('/send_request', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ receiver_username: friendUsername }),
        })
        .then(response => response.json())
        .then(data => {
            messageDiv.textContent = data.message || data.error || 'Something went wrong';
        })
        .catch(error => {
            console.error('Error sending friend request:', error);
            messageDiv.textContent = 'Something went wrong';
        });
    });
        // Event listener for accept request button
    document.querySelectorAll('.accept-request-btn').forEach(button => {
        button.addEventListener('click', function() {
            const sender_id = button.getAttribute('sender_id');
            const receiver_id = button.getAttribute('receiver_id');
            // Send AJAX request to handle acceptance
            fetch('/accept_request', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ sender_id: sender_id, receiver_id: receiver_id }),
            })
            .then(response => response.json())
            .then(data => {
                // Handle response from server
                console.log(data); // You can update the UI based on the response if needed
                message = {type: "Accepted", sender_id: sender_id, receiver_id: receiver_id}
                socket.send(JSON.stringify(message))
                document.location.href="/";
            })
            .catch(error => {
                console.error('Error accepting request:', error);
                // Handle error
            });

        });
    });
});