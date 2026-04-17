import * as React from "react";

type Props = {
    title: string
    children: React.ReactNode
}

export default function FormCard({ title, children }: Props) {
    return (
        <div className="formPage">
            <div className="formCard">

                <h1 className="formTitle">
                    {title}
                </h1>

                {children}

            </div>
        </div>
    )
}