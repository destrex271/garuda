import axios from "axios";

export async function getAPISData() {
    // Instead of the file system,
    // fetch post data from an external API endpoint
    const res = await axios.get('.http://localhost:6555/apis');
    return res.json();
  }
  