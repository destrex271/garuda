import Image from "next/image";
import { MainDashboard } from "../components/main-dashboard";

export default async function Home() {
  let data = await fetch('http://localhost:6555/apis')
  let jsonData = await data.json()

  // console.log(jsonData[1])

  return (
    <main>
      <MainDashboard data={jsonData}/>
    </main>
  );
}
