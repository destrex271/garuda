import { Details } from "../../components/details"

export default async function Page({ params }) {

    const id = params.id

    let data = await fetch('http://localhost:6555/apis')
    let dt = await data.json()

    console.log(dt)

    dt = dt.filter((dt) => (
        dt['id'] == id
    ))
    

    return (
        <Details data={dt[0]} />
    )
}